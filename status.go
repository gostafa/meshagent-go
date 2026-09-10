package meshagent

// Status describes the MeshAgent service on this machine.
type Status struct {
	// Installed reports whether the service is registered.
	Installed bool
	// Running reports whether the service is in the running state. A service
	// that is installed but not running means the device stays registered in
	// its MeshCentral device group and shows as offline.
	Running bool
	// State is the Windows service state name, for example "running",
	// "stopped" or "start pending". Empty when the service is not installed.
	State string
	// BinaryPath is the executable the service was registered with, with any
	// quoting and trailing arguments stripped. Empty when not installed.
	BinaryPath string
}
