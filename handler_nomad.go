package evacuator

import (
	"context"
	"fmt"
	"log/slog"

	nomadApi "github.com/hashicorp/nomad/api"
)

type NomadHandler struct {
	nomadClient *nomadApi.Client
	config      NomadHandlerConfig
}

type NomadHandlerConfig struct {
	Logger *slog.Logger
	Force  bool
}

func NewNomadHandler(config *NomadHandlerConfig) (*NomadHandler, error) {

	client, err := nomadApi.NewClient(nomadApi.DefaultConfig())
	if err != nil {
		config.Logger.Error("failed to create Nomad client", "error", err.Error())
		return nil, err
	}

	return &NomadHandler{
		nomadClient: client,
		config:      *config,
	}, nil
}

func (h *NomadHandler) Name() string {
	return "nomad"
}

func (h *NomadHandler) HandleTermination(ctx context.Context, event TerminationEvent) error {

	h.config.Logger.Info("handling nomad node termination", "node", event.Hostname, "handler", h.Name())

	// get nomad nodes
	nomadNodes, _, err := h.nomadClient.Nodes().List(&nomadApi.QueryOptions{
		Filter: fmt.Sprintf(`Name == "%s"`, event.Hostname),
	})

	if err != nil {
		h.config.Logger.Debug("failed to list nomad nodes", "error", err.Error(), "handler", h.Name())
		return err
	}

	// get nomad node ID for first data
	var nodeID string
	for _, node := range nomadNodes {
		if node.Name == event.Hostname {
			nodeID = node.ID
			break
		}
	}

	if nodeID == "" {
		h.config.Logger.Debug(fmt.Sprintf("failed to find nomad node for %s", event.Hostname), "handler", h.Name())
		return err
	}

	h.config.Logger.Info("nomad node found, proceeding with cordon", "node_id", nodeID, "node", nodeID, "node", event.Hostname, "handler", h.Name())

	// cordon & drain the node
	_, err = h.nomadClient.Nodes().UpdateDrain(nodeID, &nomadApi.DrainSpec{
		IgnoreSystemJobs: h.config.Force,
	}, false, &nomadApi.WriteOptions{})

	if err != nil {
		h.config.Logger.Debug(fmt.Sprintf("failed to drain nomad node for %s", event.Hostname), "handler", h.Name())
		return err
	}
	h.config.Logger.Info("nomad node successfully drained", "node_id", nodeID, "node", event.Hostname, "handler", h.Name())

	return nil
}
