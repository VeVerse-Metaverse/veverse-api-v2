package k8s

import (
	"context"
	sc "dev.hackerman.me/artheon/veverse-shared/context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	st "dev.hackerman.me/artheon/veverse-shared/telegram"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"veverse-api/helper"
)

var environment string
var namespace string
var config *rest.Config
var clientset *kubernetes.Clientset
var dynamicClient *dynamic.DynamicClient
var gameServerResource = schema.GroupVersionResource{Group: "veverse.com", Version: "v1", Resource: "gameservers"}

func Setup() (err error) {
	var isK8sCluster bool

	kubernetesServiceHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	if kubernetesServiceHost == "" {
		isK8sCluster = false
	} else {
		isK8sCluster = true
	}

	environment = os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "dev"
	}

	namespace = os.Getenv("GAMESERVER_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	if isK8sCluster {
		config, err = rest.InClusterConfig()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "unable to setup cluster config: %v\n", err)
			return err
		}
	} else {
		// fallback to local kube config if not running in cluster
		var homeDir string
		if homeDir = os.Getenv("HOME"); homeDir == "" {
			homeDir = os.Getenv("USERPROFILE") // windows
		}

		config, err = clientcmd.BuildConfigFromFlags("", fmt.Sprintf("%s/.kube/config", homeDir))
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "unable to setup cluster config: %v\n", err)
			return err
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to setup cluster client set: %v\n", err)
		return err
	}

	dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to setup cluster dynamic client: %v\n", err)
		return err
	}

	return nil
}

func NewMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.SetUserContext(context.WithValue(c.UserContext(), sc.Environment, environment))
		c.SetUserContext(context.WithValue(c.UserContext(), sc.K8sNamespace, namespace))
		//c.SetUserContext(context.WithValue(c.UserContext(), sc.K8sConfig, config))
		c.SetUserContext(context.WithValue(c.UserContext(), sc.K8sClientSet, clientset))
		c.SetUserContext(context.WithValue(c.UserContext(), sc.K8sDynamicClient, dynamicClient))
		c.SetUserContext(context.WithValue(c.UserContext(), sc.K8sGameServerResource, gameServerResource))

		return c.Next()
	}
}

func AddGameServerResource(ctx context.Context, m sm.GameServerV2) (out *unstructured.Unstructured, err error) {
	err = st.SendTelegramMessage(fmt.Sprintf("creating game server %s in namespace %s...", m.Id, namespace))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to send telegram message: %v\n", err)
	}

	apiV1Token := os.Getenv("GAMESERVER_API_V1_TOKEN")
	apiV2Email := os.Getenv("GAMESERVER_API_V2_EMAIL")
	apiV2Password := os.Getenv("GAMESERVER_API_V2_PASS")

	var apiV2Token string
	apiV2Token, err = helper.LoginInternal(ctx, apiV2Email, apiV2Password)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to login to gameserver api v2: %v\n", err)
		_ = st.SendTelegramMessage(fmt.Sprintf("unable to login to gameserver api v2: %v", err))
		return nil, err
	}

	ctx = context.WithValue(ctx, sc.GameServerApiV1Token, apiV1Token)
	ctx = context.WithValue(ctx, sc.GameServerApiV2Token, apiV2Token)

	u, err := m.ToUnstructured(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to convert game server to unstructured: %v\n", err)
		_ = st.SendTelegramMessage(fmt.Sprintf("unable to convert game server to unstructured: %v", err))
		return nil, err
	}

	gameServer, err := dynamicClient.Resource(gameServerResource).Namespace(namespace).Create(ctx, &u, metaV1.CreateOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to create game server: %v\n", err)
		_ = st.SendTelegramMessage(fmt.Sprintf("unable to create game server: %v", err))
		return nil, err
	}

	return gameServer, nil
}

func DeleteGameServerResource(ctx context.Context, id uuid.UUID) (err error) {
	name := fmt.Sprintf("gs-%s", id)

	err = st.SendTelegramMessage(fmt.Sprintf("deleting game server %s in namespace %s...", id, namespace))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to send telegram message: %v\n", err)
	}

	err = dynamicClient.Resource(gameServerResource).Namespace(namespace).Delete(ctx, name, metaV1.DeleteOptions{})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to delete game server: %v\n", err)
		return err
	}

	return nil
}
