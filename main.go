package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"

	"fmt"
	// "net/http"
	"os"
	"os/signal"
	"syscall"

	wh "gpu-resource-toleration-admission-controller/webhook"
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
		log.Printf("Failed to load key pair: %s\n", err)
	}

	webhookServer := wh.GetAdmissionWebhookServer(keyPair, port)

	fmt.Println("Starting xx webhook server...")

	go func() {
		if err := webhookServer.ListenAndServeTLS("", ""); err != nil {
			log.Printf("Failed to listen and serve webhook server: %s\n", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("OS shutdown signal received...")
	webhookServer.Shutdown(context.Background())
}
