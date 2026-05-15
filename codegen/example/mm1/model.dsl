ENTITY 1 {
  Order {
  }
}

VAR 2 {
  arrivalRate float64 1.0
  serviceRate float64 1.25
}

RESOURCE 0 {
}

BLOCK 4 {
  CREATE Source {
    entity Order
    interval exponential(arrivalRate)
    next WaitQueue
    blocking_policy destroy
  }


  QUEUE WaitQueue {
    capacity infinity
    next ServiceDelay
  }

  DELAY ServiceDelay {
    duration exponential(serviceRate)
    capacity 1
    wait_queue WaitQueue
    next Sink
  }

  TERMINATE Sink {
  }

}