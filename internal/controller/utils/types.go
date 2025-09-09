package utils

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Dependencies holds shared dependencies for controllers.
type Dependencies struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
	Logger *zap.SugaredLogger
}
