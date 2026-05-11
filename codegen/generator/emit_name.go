package generator

func goType(t ValueType) string {
	switch t {
	case ValueFloat64:
		return "float64"
	case ValueInt64:
		return "int64"
	case ValueUint64:
		return "uint64"
	case ValueBool:
		return "bool"
	default:
		return "unknown"
	}
}

func entityTypeName(entity *EntityDesc) string {
	return "entityType_" + entity.Name
}

func entityStructName(entity *EntityDesc) string {
	return "entity_" + entity.Name
}

func entityPoolName(entity *EntityDesc) string {
	return "pool_entity_" + entity.Name
}

func entityFieldName(field *FieldDesc) string {
	return "field_" + field.Name
}

func varGoName(v *VarDesc) string {
	return "var_" + v.Name
}

func resourceGoName(resource *ResourceDesc) string {
	return "resource_" + resource.Name
}

func resourceSeizeFuncName(resource *ResourceDesc) string {
	return "func_seize_resource_" + resource.Name
}

func resourceReleaseFuncName(resource *ResourceDesc) string {
	return "func_release_resource_" + resource.Name
}

func blockKindName(block *BlockDesc) string {
	switch block.Params.(type) {
	case *CreateParams:
		return "CREATE"
	case *QueueParams:
		return "QUEUE"
	case *DelayParams:
		return "DELAY"
	case *SeizeParams:
		return "SEIZE"
	case *ReleaseParams:
		return "RELEASE"
	case *AssignParams:
		return "ASSIGN"
	case *BranchParams:
		return "BRANCH"
	case *TerminateParams:
		return "TERMINATE"
	default:
		return "UNKNOWN"
	}
}

func blockStructName(block *BlockDesc) string {
	return "struct_block_" + blockKindName(block) + "_" + block.Name
}

func blockGoName(block *BlockDesc) string {
	return "block_" + blockKindName(block) + "_" + block.Name
}

func blockFuncName(block *BlockDesc) string {
	return "func_block_" + blockKindName(block) + "_" + block.Name
}
