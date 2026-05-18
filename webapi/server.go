package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	cpb "github.com/gulyaev-iv/go-cloud-des/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	cfg          Config
	controlPlane *ControlPlaneClient
	store        *S3Store
}

func NewServer(cfg Config, controlPlane *ControlPlaneClient, store *S3Store) *Server {
	return &Server{
		cfg:          cfg,
		controlPlane: controlPlane,
		store:        store,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/batches", s.handleBatches)
	mux.HandleFunc("/api/batches/", s.handleBatch)
	mux.HandleFunc("/api/models/", s.handleModelExperiment)
	mux.HandleFunc("/api/nodes", s.handleNodes)

	var handler http.Handler = mux

	if s.cfg.EnableCORS {
		handler = s.cors(handler)
	}

	return handler
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.CORSAllowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleBatches(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/batches" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req cpb.SubmitExperimentBatchRequest
	if err := readProtoJSON(w, r, s.cfg.MaxRequestBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := s.controlPlane.SubmitExperimentBatch(r.Context(), &req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusAccepted, resp)
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	batchID, ok := singlePathParam(r.URL.Path, "/api/batches/")
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	resp, err := s.controlPlane.GetExperimentBatch(r.Context(), batchID)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/nodes" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	statusFilter := r.URL.Query().Get("status")

	resp, err := s.controlPlane.ListNodes(r.Context(), statusFilter)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeProtoJSON(w, http.StatusOK, resp)
}

func (s *Server) handleModelExperiment(w http.ResponseWriter, r *http.Request) {
	modelHash, experimentID, action, ok := parseExperimentPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch action {
	case "":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		resp, err := s.controlPlane.GetExperiment(r.Context(), modelHash, experimentID)
		if err != nil {
			writeGRPCError(w, err)
			return
		}

		writeProtoJSON(w, http.StatusOK, resp)

	case "cancel":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		resp, err := s.controlPlane.CancelExperiment(r.Context(), modelHash, experimentID)
		if err != nil {
			writeGRPCError(w, err)
			return
		}

		writeProtoJSON(w, http.StatusOK, resp)

	case "metrics.csv":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		s.handleMetricsCSV(w, r, modelHash, experimentID)

	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleMetricsCSV(w http.ResponseWriter, r *http.Request, modelHash string, experimentID string) {
	experiment, err := s.controlPlane.GetExperiment(r.Context(), modelHash, experimentID)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	artifact := experiment.GetMetricsArtifact()
	if artifact == nil || artifact.GetKey() == "" {
		writeError(w, http.StatusNotFound, "metrics artifact not found")
		return
	}

	object, info, err := s.store.OpenObject(r.Context(), artifact.GetKey())
	if err != nil {
		log.Printf(
			"open metrics csv failed: model_hash=%s experiment_id=%s key=%s error=%v",
			modelHash,
			experimentID,
			artifact.GetKey(),
			err,
		)

		writeError(w, http.StatusBadGateway, "failed to open metrics artifact")
		return
	}
	defer object.Close()

	contentType := artifact.GetContentType()
	if contentType == "" {
		contentType = info.ContentType
	}
	if contentType == "" {
		contentType = "text/csv; charset=utf-8"
	}

	size := artifact.GetSize()
	if size <= 0 {
		size = info.Size
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, csvFileName(modelHash, experimentID)))

	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}

	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, object); err != nil {
		log.Printf(
			"stream metrics csv failed: model_hash=%s experiment_id=%s key=%s error=%v",
			modelHash,
			experimentID,
			artifact.GetKey(),
			err,
		)
	}
}

func readProtoJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, msg proto.Message) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	defer r.Body.Close()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("empty request body")
	}

	return protojson.UnmarshalOptions{
		DiscardUnknown: false,
	}.Unmarshal(data, msg)
}

func writeProtoJSON(w http.ResponseWriter, statusCode int, msg proto.Message) {
	data, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}.Marshal(msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if _, err := w.Write(data); err != nil {
		log.Printf("write response failed: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if _, err := w.Write(data); err != nil {
		log.Printf("write response failed: %v", err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeError(w, grpcCodeToHTTP(st.Code()), st.Message())
}

func grpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	default:
		return http.StatusBadGateway
	}
}

func singlePathParam(pathValue string, prefix string) (string, bool) {
	raw := strings.TrimPrefix(pathValue, prefix)
	if raw == "" || raw == pathValue || strings.Contains(raw, "/") {
		return "", false
	}

	value, err := url.PathUnescape(raw)
	if err != nil || value == "" {
		return "", false
	}

	return value, true
}

func parseExperimentPath(pathValue string) (modelHash string, experimentID string, action string, ok bool) {
	raw := strings.TrimPrefix(pathValue, "/api/models/")
	if raw == pathValue || raw == "" {
		return "", "", "", false
	}

	parts := strings.Split(raw, "/")
	if len(parts) != 3 && len(parts) != 4 {
		return "", "", "", false
	}

	if parts[1] != "experiments" {
		return "", "", "", false
	}

	modelHash, err := url.PathUnescape(parts[0])
	if err != nil || modelHash == "" {
		return "", "", "", false
	}

	experimentID, err = url.PathUnescape(parts[2])
	if err != nil || experimentID == "" {
		return "", "", "", false
	}

	if len(parts) == 4 {
		action, err = url.PathUnescape(parts[3])
		if err != nil || action == "" {
			return "", "", "", false
		}
	}

	return modelHash, experimentID, action, true
}

func csvFileName(modelHash string, experimentID string) string {
	modelShort := modelHash
	if len(modelShort) > 12 {
		modelShort = modelShort[:12]
	}

	name := "metrics_" + modelShort + "_" + experimentID + ".csv"

	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"\"", "_",
		"\r", "_",
		"\n", "_",
	)

	return replacer.Replace(name)
}
