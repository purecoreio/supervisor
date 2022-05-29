package main

type Storage struct {
	machine *Machine // the hosting machine
	path    string   // the path on the original machine the storage is mounted on
	size    uint64   // the total size this path can allocate
}

func (s Storage) GetUsedSpace() (used uint64) {
	// no need to compute used space, take into account reserved space for every container
	used = 0
	for _, container := range s.machine.containers {
		if container.localId != nil {
			used += container.template.size
		}
	}
	return used
}

func (s Storage) GetAvailableSpace() (available uint64) {
	return s.size - s.GetUsedSpace()
}
