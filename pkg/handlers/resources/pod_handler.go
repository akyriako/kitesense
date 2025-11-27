package resources

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/zxh326/kite/pkg/cluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	metricsv1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodHandler struct {
	*GenericResourceHandler[*corev1.Pod, *corev1.PodList]
}

func NewPodHandler() *PodHandler {
	return &PodHandler{
		GenericResourceHandler: NewGenericResourceHandler[*corev1.Pod, *corev1.PodList]("pods", false, true),
	}
}

type PodMetrics struct {
	CPUUsage      int64 `json:"cpuUsage,omitempty"`
	CPULimit      int64 `json:"cpuLimit,omitempty"`
	CPURequest    int64 `json:"cpuRequest,omitempty"`
	MemoryUsage   int64 `json:"memoryUsage,omitempty"`
	MemoryLimit   int64 `json:"memoryLimit,omitempty"`
	MemoryRequest int64 `json:"memoryRequest,omitempty"`
}

type PodWithMetrics struct {
	*corev1.Pod `json:",inline"`
	Metrics     *PodMetrics `json:"metrics"`
}

type PodListWithMetrics struct {
	Items           []*PodWithMetrics `json:"items"`
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
}

func GetPodMetrics(metricsMap map[string]metricsv1.PodMetrics, pod *corev1.Pod) *PodMetrics {
	key := pod.Namespace + "/" + pod.Name
	podMetrics, ok := metricsMap[key]
	if !ok || len(podMetrics.Containers) == 0 {
		return nil
	}
	var cpuUsage, memUsage int64
	for _, container := range podMetrics.Containers {
		if cpuQuantity, ok := container.Usage["cpu"]; ok {
			cpuUsage += cpuQuantity.MilliValue()
		}
		if memQuantity, ok := container.Usage["memory"]; ok {
			memUsage += memQuantity.Value()
		}
	}
	var cpuLimit, memLimit int64
	var cpuRequest, memRequest int64
	for _, container := range pod.Spec.Containers {
		if cpuQuantity, ok := container.Resources.Limits["cpu"]; ok {
			cpuLimit += cpuQuantity.MilliValue()
		}
		if memQuantity, ok := container.Resources.Limits["memory"]; ok {
			memLimit += memQuantity.Value()
		}
		if cpuQuantity, ok := container.Resources.Requests["cpu"]; ok {
			cpuRequest += cpuQuantity.MilliValue()
		}
		if memQuantity, ok := container.Resources.Requests["memory"]; ok {
			memRequest += memQuantity.Value()
		}
	}
	return &PodMetrics{
		CPUUsage:      cpuUsage,
		MemoryUsage:   memUsage,
		CPULimit:      cpuLimit,
		MemoryLimit:   memLimit,
		CPURequest:    cpuRequest,
		MemoryRequest: memRequest,
	}
}

func (h *PodHandler) ListMetrics(c *gin.Context) (map[string]metricsv1.PodMetrics, error) {
	cs := c.MustGet("cluster").(*cluster.ClientSet)
	var metricsList metricsv1.PodMetricsList
	var listOpts []client.ListOption
	if namespace := c.Param("namespace"); namespace != "" && namespace != "_all" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}
	if labelSelector := c.Query("labelSelector"); labelSelector != "" {
		selector, err := metav1.ParseToLabelSelector(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid labelSelector parameter: %w", err)
		}
		labelSelectorOption, err := metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			return nil, fmt.Errorf("failed to convert labelSelector: %w", err)
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: labelSelectorOption})
	}
	if err := cs.K8sClient.List(c, &metricsList, listOpts...); err != nil {
		klog.Warningf("Failed to list pod metrics: %v", err)
	}

	metricsMap := lo.KeyBy(metricsList.Items, func(item metricsv1.PodMetrics) string {
		return item.Namespace + "/" + item.Name
	})

	return metricsMap, nil
}

func (h *PodHandler) List(c *gin.Context) {
	objlist, err := h.list(c)
	if err != nil {
		return
	}
	reduce := c.Query("reduce") == "true"
	metricsMap, err := h.ListMetrics(c)
	if err != nil {
		klog.Warningf("Failed to list pod metrics: %v", err)
	}

	result := &PodListWithMetrics{
		TypeMeta: objlist.TypeMeta,
		ListMeta: objlist.ListMeta,
		Items:    make([]*PodWithMetrics, len(objlist.Items)),
	}

	for i := range objlist.Items {
		item := &PodWithMetrics{
			Pod: &objlist.Items[i],
		}
		item.Metrics = GetPodMetrics(metricsMap, &objlist.Items[i])
		if reduce {
			// remove unnecessary fields to reduce response size
			item.ObjectMeta = metav1.ObjectMeta{
				Name:              item.Name,
				Namespace:         item.Namespace,
				CreationTimestamp: item.CreationTimestamp,
				DeletionTimestamp: item.DeletionTimestamp,
			}
			item.Spec = corev1.PodSpec{
				NodeName: objlist.Items[i].Spec.NodeName,
				InitContainers: lo.Map(objlist.Items[i].Spec.InitContainers, func(c corev1.Container, _ int) corev1.Container {
					return corev1.Container{Name: c.Name, Image: c.Image, RestartPolicy: c.RestartPolicy}
				}),
				Containers: lo.Map(objlist.Items[i].Spec.Containers, func(c corev1.Container, _ int) corev1.Container {
					return corev1.Container{Name: c.Name, Image: c.Image, RestartPolicy: c.RestartPolicy}
				}),
			}
		}
		result.Items[i] = item
	}
	c.JSON(200, result)
}

// registerCustomRoutes adds pod-specific extra routes (SSE watch)
func (h *PodHandler) registerCustomRoutes(group *gin.RouterGroup) {
	// watch pods in namespace (or _all)
	group.GET("/:namespace/watch", h.Watch)
	group.GET("/:namespace/health/watch", h.WatchHealth)
}

// writeSSE writes a single SSE event with the given name and payload
func writeSSE(c *gin.Context, event string, payload any) error {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	// Try to stream chunked
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// Watch implements SSE-based watch for pods list with initial snapshot and incremental updates
func (h *PodHandler) Watch(c *gin.Context) {
	cs := c.MustGet("cluster").(*cluster.ClientSet)

	// Parse params
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "_all"
	}
	reduce := c.DefaultQuery("reduce", "false") == "true"
	labelSelector := c.Query("labelSelector")
	fieldSelector := c.Query("fieldSelector")

	listOpts := metav1.ListOptions{}
	if labelSelector != "" {
		listOpts.LabelSelector = labelSelector
	}
	if fieldSelector != "" {
		listOpts.FieldSelector = fieldSelector
	}

	ns := namespace
	if ns == "_all" {
		ns = ""
	}
	metricsMap, err := h.ListMetrics(c)
	if err != nil {
		klog.Warningf("Failed to list pod metrics: %v", err)
	}

	watchInterface, err := cs.K8sClient.ClientSet.CoreV1().Pods(ns).Watch(c, listOpts)
	if err != nil {
		_ = writeSSE(c, "error", gin.H{"error": fmt.Sprintf("failed to start watch: %v", err)})
		return
	}
	defer watchInterface.Stop()

	// Keep-alive pings
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	flusher, _ := c.Writer.(http.Flusher)

	for {
		select {
		case <-c.Request.Context().Done():
			_ = writeSSE(c, "close", gin.H{"message": "connection closed"})
			return
		case <-ticker.C:
			metricsMap, _ = h.ListMetrics(c)
			for _, metrics := range metricsMap {
				pod, err := h.GetResource(c, metrics.Namespace, metrics.Name)
				if err != nil {
					klog.Warningf("Failed to get pod: %v", err)
					continue
				}
				p := pod.(*corev1.Pod)
				obj := &PodWithMetrics{Pod: p, Metrics: GetPodMetrics(metricsMap, p)}
				_ = writeSSE(c, "modified", obj)
			}
			_, _ = fmt.Fprintf(c.Writer, ": ping\n\n") // comment line per SSE
			flusher.Flush()
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				_ = writeSSE(c, "close", gin.H{"message": "watch channel closed"})
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok || pod == nil {
				continue
			}

			obj := &PodWithMetrics{Pod: pod}
			if reduce {
				obj.Pod = pod.DeepCopy()
				obj.ObjectMeta = metav1.ObjectMeta{
					Name:              pod.Name,
					Namespace:         pod.Namespace,
					CreationTimestamp: pod.CreationTimestamp,
					DeletionTimestamp: pod.DeletionTimestamp,
				}
				obj.Spec = corev1.PodSpec{
					NodeName: pod.Spec.NodeName,
					InitContainers: lo.Map(pod.Spec.InitContainers, func(c corev1.Container, _ int) corev1.Container {
						return corev1.Container{Name: c.Name, Image: c.Image, RestartPolicy: c.RestartPolicy}
					}),
					Containers: lo.Map(pod.Spec.Containers, func(c corev1.Container, _ int) corev1.Container {
						return corev1.Container{Name: c.Name, Image: c.Image, RestartPolicy: c.RestartPolicy}
					}),
				}
			}
			obj.Metrics = GetPodMetrics(metricsMap, pod)
			switch event.Type {
			case watch.Added:
				_ = writeSSE(c, "added", obj)
			case watch.Modified:
				_ = writeSSE(c, "modified", obj)
			case watch.Deleted:
				_ = writeSSE(c, "deleted", obj)
			case watch.Error:
				_ = writeSSE(c, "error", gin.H{"error": "watch error"})
			default:
				// ignore
			}
		}
	}
}

// Typesense Pods Healthcheck

type PodHealthStatus struct {
	PodName        string    `json:"podName"`
	Namespace      string    `json:"namespace"`
	State          string    `json:"state"` // LEADER, FOLLOWER, CANDIDATE, UNKNOWN
	Healthy        bool      `json:"healthy"`
	CommittedIndex int64     `json:"committedIndex"`
	QueuedWrites   int64     `json:"queuedWrites"`
	Timestamp      time.Time `json:"timestamp"`
	Error          string    `json:"error,omitempty"`
}

type HealthCheckResponse struct {
	ClusterStatus    string                      `json:"cluster_status"`
	ClusterHealth    bool                        `json:"cluster_health"`
	NodesHealthCheck map[string]NodeHealthDetail `json:"nodes_health_check"`
}

type NodeHealthDetail struct {
	NodeStatus NodeStateInfo  `json:"node_status"`
	NodeHealth NodeHealthInfo `json:"node_health"`
}

type NodeStateInfo struct {
	CommittedIndex int64  `json:"committed_index"`
	QueuedWrites   int64  `json:"queued_writes"`
	State          string `json:"state"`
}

type NodeHealthInfo struct {
	Ok bool `json:"ok"`
}

// WatchHealth implements SSE-based health check monitoring for pods
func (h *PodHandler) WatchHealth(c *gin.Context) {
	cs := c.MustGet("cluster").(*cluster.ClientSet)

	// Parse params
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = "_all"
	}

	labelSelector := c.Query("labelSelector")
	healthEndpoint := c.DefaultQuery("healthEndpoint", "localhost:8088/readyz")

	listOpts := metav1.ListOptions{}
	if labelSelector != "" {
		listOpts.LabelSelector = labelSelector
	}

	ns := namespace
	if ns == "_all" {
		ns = ""
	}

	// Watch pods
	watchInterface, err := cs.K8sClient.ClientSet.CoreV1().Pods(ns).Watch(c, listOpts)
	if err != nil {
		_ = writeSSE(c, "error", gin.H{"error": fmt.Sprintf("failed to start watch: %v", err)})
		return
	}
	defer watchInterface.Stop()

	// Track current pod health states
	podHealthMap := make(map[string]*PodHealthStatus)

	// Initial health check on all pods
	podList, err := cs.K8sClient.ClientSet.CoreV1().Pods(ns).List(c, listOpts)
	if err == nil {
		for i := range podList.Items {
			pod := &podList.Items[i]
			key := pod.Namespace + "/" + pod.Name

			health := h.checkPodHealth(c, pod, healthEndpoint)
			podHealthMap[key] = health

			_ = writeSSE(c, "snapshot", podHealthMap)
		}
	}

	// Keep-alive and periodic health check
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	flusher, _ := c.Writer.(http.Flusher)

	for {
		select {
		case <-c.Request.Context().Done():
			_ = writeSSE(c, "close", gin.H{"message": "connection closed"})
			return

		case <-ticker.C:
			// Periodic health check on all pods
			podList, err := cs.K8sClient.ClientSet.CoreV1().Pods(ns).List(c, listOpts)
			if err != nil {
				klog.Warningf("Failed to list pods for health check: %v", err)
				continue
			}

			for i := range podList.Items {
				pod := &podList.Items[i]
				key := pod.Namespace + "/" + pod.Name

				// Check health and compare with previous state
				newHealth := h.checkPodHealth(c, pod, healthEndpoint)

				// Only send update if state changed
				if oldHealth, exists := podHealthMap[key]; !exists || hasHealthChanged(oldHealth, newHealth) {
					podHealthMap[key] = newHealth
					_ = writeSSE(c, "updated", newHealth)
				}
			}

			_, _ = fmt.Fprintf(c.Writer, ": ping\n\n")
			flusher.Flush()

		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				_ = writeSSE(c, "close", gin.H{"message": "watch channel closed"})
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok || pod == nil {
				continue
			}

			key := pod.Namespace + "/" + pod.Name

			switch event.Type {
			case watch.Added:
				health := h.checkPodHealth(c, pod, healthEndpoint)
				podHealthMap[key] = health
				_ = writeSSE(c, "updated", health)

			case watch.Modified:
				health := h.checkPodHealth(c, pod, healthEndpoint)
				if oldHealth, exists := podHealthMap[key]; !exists || hasHealthChanged(oldHealth, health) {
					podHealthMap[key] = health
					_ = writeSSE(c, "updated", health)
				}

			case watch.Deleted:
				if _, exists := podHealthMap[key]; exists {
					delete(podHealthMap, key)
					_ = writeSSE(c, "removed", gin.H{
						"podName":   pod.Name,
						"namespace": pod.Namespace,
					})
				}
			}
		}
	}
}

// checkPodHealth probes a pod's health endpoint and extracts status
func (h *PodHandler) checkPodHealth(c *gin.Context, pod *corev1.Pod, endpoint string) *PodHealthStatus {
	status := &PodHealthStatus{
		PodName:   pod.Name,
		Namespace: pod.Namespace,
		Timestamp: time.Now(),
		State:     "UNKNOWN",
		Healthy:   false,
	}

	if pod.Status.PodIP == "" {
		status.Error = "Pod IP not assigned"
		return status
	}

	url := fmt.Sprintf("http://%s:8808/readyz", pod.Status.PodIP)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		status.Error = fmt.Sprintf("Health check failed: %v", err)
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return status
	}

	var healthResp HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		status.Error = fmt.Sprintf("Failed to parse response: %v", err)
		return status
	}

	podFQDN := pod.Name + "." + pod.Name + "-sts-svc"
	if nodeCheck, ok := healthResp.NodesHealthCheck[podFQDN]; ok {
		status.State = nodeCheck.NodeStatus.State
		status.CommittedIndex = nodeCheck.NodeStatus.CommittedIndex
		status.QueuedWrites = nodeCheck.NodeStatus.QueuedWrites
		status.Healthy = nodeCheck.NodeHealth.Ok
	}

	if !healthResp.ClusterHealth {
		status.Error = "Cluster unhealthy"
	}

	return status
}

func hasHealthChanged(old, new *PodHealthStatus) bool {
	return old.State != new.State ||
		old.Healthy != new.Healthy ||
		old.Error != new.Error
}
