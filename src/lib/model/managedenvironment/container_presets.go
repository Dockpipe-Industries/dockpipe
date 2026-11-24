package managedenvironment

type ContainerPreset struct {
	Name    string
	Image   string
	Startup string
}

func ContainerPresets() []ContainerPreset {
	return []ContainerPreset{
		{Name: "ubuntu", Image: "ubuntu:24.04", Startup: "keep-running"},
		{Name: "debian", Image: "debian:13-slim", Startup: "keep-running"},
		{Name: "alpine", Image: "alpine:3.22", Startup: "keep-running"},
		{Name: "nginx", Image: "nginx:alpine", Startup: "image"},
	}
}
