package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	wh "gpu-resource-toleration-admission-controller/webhook"

	"k8s.io/klog"
)

func main() {
	var port int
	var certFile string
	var keyFile string
	var targetResources wh.ArrayFlags

	flag.IntVar(&port, "port", 8443, "webhook server port")
	flag.Var(&targetResources, "targetResource", "target resource to add taints")
	flag.StringVar(&certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "x509 Certificate file for TLS connection")
	flag.StringVar(&keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "x509 Private key file for TLS connection")
	flag.Parse()

	wh.SetTargetResourcesSet(targetResources)

	keyPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		klog.Errorf("Failed to load key pair: %s", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", wh.HandleMutate)
	mux.HandleFunc("/validate", wh.HandleValidate)

	webhookServer := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{keyPair}},
	}

	klog.Info("Starting xx webhook server...")

	go func() {
		if err := webhookServer.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("Failed to listen and serve webhook server: %s", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	klog.Info("OS shutdown signal received...")
	webhookServer.Shutdown(context.Background())
}
