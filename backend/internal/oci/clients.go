package oci

import (
	"errors"
	"os"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/workrequests"
)

var ErrNotReady = errors.New("oci configuration is not ready")

type Clients struct {
	Identity       identity.IdentityClient
	Compute        core.ComputeClient
	VirtualNetwork core.VirtualNetworkClient
	WorkRequests   workrequests.WorkRequestClient
}

func NewClients(cfg ReadinessConfig) (Clients, error) {
	readiness := CheckReadiness(cfg)
	if !readiness.Ready {
		return Clients{}, ErrNotReady
	}

	privateKey, err := privateKeyValue(cfg)
	if err != nil {
		return Clients{}, err
	}

	provider := common.NewRawConfigurationProvider(
		cfg.TenancyOCID,
		cfg.UserOCID,
		cfg.Region,
		cfg.Fingerprint,
		privateKey,
		nil,
	)

	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		return Clients{}, err
	}
	computeClient, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		return Clients{}, err
	}
	networkClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(provider)
	if err != nil {
		return Clients{}, err
	}
	workRequestClient, err := workrequests.NewWorkRequestClientWithConfigurationProvider(provider)
	if err != nil {
		return Clients{}, err
	}

	return Clients{
		Identity:       identityClient,
		Compute:        computeClient,
		VirtualNetwork: networkClient,
		WorkRequests:   workRequestClient,
	}, nil
}

func privateKeyValue(cfg ReadinessConfig) (string, error) {
	if cfg.PrivateKey != "" {
		return cfg.PrivateKey, nil
	}
	raw, err := os.ReadFile(cfg.PrivateKeyFile)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
