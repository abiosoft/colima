package apple

// Dependencies returns the dependencies required for Apple Container runtime.
// Only docker is required as an external dependency.
// The container CLI is checked as a VM dependency.
// Socktainer is installed automatically when starting the daemon.
func (a appleRuntime) Dependencies() []string {
	return []string{"docker"}
}