package main

type Hardware struct {
	machine   *Machine
	CPU       string
	coreCount uint
	memory    uint64
}

func (h Hardware) GetUsedCores() (used float32) {
	// no need to compute used space, take into account reserved space for every container
	used = 0
	for _, container := range h.machine.containers {
		if container.localId != nil {
			used += container.template.cores
		}
	}
	return used
}

func (h Hardware) GetAvailableCores() (available float32) {
	return float32(h.coreCount) - h.GetUsedCores()
}

func (h Hardware) GetUsedMemory() (used uint64) {
	// no need to compute used space, take into account reserved space for every container
	used = 0
	for _, container := range h.machine.containers {
		if container.localId != nil {
			used += container.template.memory
		}
	}
	return used
}

func (h Hardware) GetAvailableMemory() (available uint64) {
	return h.memory - h.GetUsedMemory()
}
