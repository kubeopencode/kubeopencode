// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// RunConnectionHeartbeat periodically patches the connection heartbeat annotation
// on the Agent to prevent standby auto-suspend. It updates immediately on start,
// then every ConnectionHeartbeatInterval until the context is cancelled.
//
// Uses raw merge patch to avoid read-modify-write race conditions when multiple
// connections heartbeat the same Agent concurrently.
//
// onError is called on patch failures (when ctx is not cancelled). Callers use
// this to log or warn as appropriate for their context.
func RunConnectionHeartbeat(ctx context.Context, k8sClient client.Client, namespace, agentName string, onError func(error)) {
	patchOnce := func() {
		agent := &kubeopenv1alpha1.Agent{}
		agent.Name = agentName
		agent.Namespace = namespace
		patchData := fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`,
			AnnotationLastConnectionActive,
			time.Now().UTC().Format(time.RFC3339))
		if err := k8sClient.Patch(ctx, agent, client.RawPatch(types.MergePatchType, []byte(patchData))); err != nil {
			if ctx.Err() == nil && onError != nil {
				onError(err)
			}
		}
	}

	patchOnce()

	ticker := time.NewTicker(ConnectionHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			patchOnce()
		}
	}
}
