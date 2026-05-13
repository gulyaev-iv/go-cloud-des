ENTITY 1 {
  Jobs {
    JobType int64
    ReworkCount int64 
  }
}

VAR 6 {
  totalEntitiesEnd uint64 10000000
  totalEntitiesSink uint64 0

  arrivalRate float64 1.0
  serviceARate float64 0.83333333
  serviceBRate float64 0.55555556
  serviceQCRate float64 2.0
}

RESOURCE 0 {
}

BLOCK 18 {
  CREATE Source {
    entity Jobs
    interval exponential(arrivalRate)
    next BranchJobType
    blocking_policy destroy
  }

  BRANCH BranchJobType {
    if uniform(0.0, 1.0) < 0.6 : AssignJobTypeOne
    else : AssignJobTypeTwo
  }

  ASSIGN AssignJobTypeOne {
    set Jobs.JobType = 1
    next BranchByType
  }

  ASSIGN AssignJobTypeTwo {
    set Jobs.JobType = 2
    next BranchByType
  }

  BRANCH BranchByType {
    if Jobs.JobType == 1 : QueueA
    if Jobs.JobType == 2 : QueueB
  }

  QUEUE QueueA {
    capacity infinity
    next DelayA
  }

  DELAY DelayA {
    capacity 1
    duration exponential(serviceARate)
    wait_queue QueueA
    next QueueQC
  }

  QUEUE QueueB {
    capacity infinity
    next DelayB
  }

  DELAY DelayB {
    capacity 1
    duration exponential(serviceBRate)
    wait_queue QueueB
    next QueueQC
  }

  QUEUE QueueQC {
    capacity infinity
    next DelayQC
  }

  DELAY DelayQC {
    capacity 1
    duration exponential(serviceQCRate)
    wait_queue QueueQC
    next QCResult
  }

  BRANCH QCResult {
    if uniform(0.0, 1.0) < 0.9 : SinkSuccessCount
    else : ReworkCountUp
  }

  ASSIGN SinkSuccessCount {
    set totalEntitiesSink = totalEntitiesSink + 1
    next SinkSuccess
  }

  TERMINATE SinkSuccess {
  }

  ASSIGN ReworkCountUp {
    set Jobs.ReworkCount = Jobs.ReworkCount + 1
    next ReworkDecision
  }

  BRANCH ReworkDecision {
    if Jobs.ReworkCount <= 3 : BranchByType
    else : SinkScrapCount
  }

  ASSIGN SinkScrapCount {
    set totalEntitiesSink = totalEntitiesSink + 1
    next SinkScrap
  }

  TERMINATE SinkScrap {
  }

}