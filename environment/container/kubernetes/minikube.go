package kubernetes

import (
	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"runtime"
)

func installMinikube(guest environment.GuestActions, r *cli.ActiveCommandChain, kubeVersion string) {
	installMinikubeCache(guest, r, kubeVersion)

	// install minikube last to ensure it is the last step
	downloadPath := "/tmp/minikube"
	url := "https://storage.googleapis.com/minikube/releases/latest/minikube-linux-" + runtime.GOARCH
	r.Add(func() error {
		return guest.RunInteractive("curl", "-L", "-#", "-o", downloadPath, url)
	})
	r.Add(func() error {
		return guest.Run("sudo", "install", downloadPath, "/usr/local/bin/minikube")
	})
}

func installMinikubeCache(guest environment.GuestActions, r *cli.ActiveCommandChain, kubeVersion string) {
	downloadPath := "/tmp/minikube-cache.tar.gz"
	url := "https://dl.k8s.io/" + kubeVersion + "/kubernetes-node-linux-" + runtime.GOARCH + ".tar.gz"
	r.Add(func() error {
		return guest.RunInteractive("curl", "-L", "-#", "-o", downloadPath, url)
	})
	r.Add(func() error {
		return guest.Run("tar", "xvfz", downloadPath, "-C", "/tmp")
	})
	r.Add(func() error {
		return guest.Run("sh", "-c", "mkdir -p $HOME/.minikube/cache/linux/"+kubeVersion)
	})
	r.Add(func() error {
		return guest.Run("sh", "-c", "cp /tmp/kubernetes/node/bin/* $HOME/.minikube/cache/linux/"+kubeVersion)
	})
}
