package internal

import (
	"context"
	"fmt"
	"log/slog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func Run() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("building in-cluster config: %w", err)
	}
	k8s, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}
	secrets := k8s.CoreV1().Secrets(cfg.Namespace)

	g := NewGenerator(slog.Default(), secrets, cfg)
	g.Generate(context.Background())

	return err
}
