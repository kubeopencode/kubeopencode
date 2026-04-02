// Copyright Contributors to the KubeOpenCode project

// Package types provides API types for the KubeOpenCode server.
package types

import (
	"time"
)

// ServerInfo represents server information
type ServerInfo struct {
	Version string `json:"version"`
}

// NamespaceList represents a list of namespaces
type NamespaceList struct {
	Namespaces []string `json:"namespaces"`
}

// AgentReference represents a reference to an Agent
type AgentReference struct {
	Name string `json:"name"`
}

// ContextItem represents a context item in the API
type ContextItem struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	MountPath   string `json:"mountPath,omitempty"`
}

// AgentTemplateReference represents a reference to an AgentTemplate
type AgentTemplateReference struct {
	Name string `json:"name"`
}

// CreateTaskRequest represents a request to create a task
type CreateTaskRequest struct {
	Name        string                  `json:"name,omitempty"`
	Description string                  `json:"description,omitempty"`
	AgentRef    *AgentReference         `json:"agentRef,omitempty"`
	TemplateRef *AgentTemplateReference `json:"templateRef,omitempty"`
	Contexts    []ContextItem           `json:"contexts,omitempty"`
}

// CreateAgentRequest represents a request to create an agent
type CreateAgentRequest struct {
	Name               string          `json:"name"`
	Profile            string          `json:"profile,omitempty"`
	TemplateRef        *AgentReference `json:"templateRef,omitempty"`
	WorkspaceDir       string          `json:"workspaceDir,omitempty"`
	ServiceAccountName string          `json:"serviceAccountName,omitempty"`

	// P0: Images (required when no template)
	AgentImage    string `json:"agentImage,omitempty"`
	ExecutorImage string `json:"executorImage,omitempty"`

	// P1: Common configuration
	MaxConcurrentTasks *int32       `json:"maxConcurrentTasks,omitempty"`
	Standby            *StandbyInfo `json:"standby,omitempty"`
	Persistence        *CreatePersistenceConfig `json:"persistence,omitempty"`

	// P2: Advanced configuration
	Port  *int32          `json:"port,omitempty"`
	Proxy *ProxyConfigInfo `json:"proxy,omitempty"`
}

// CreatePersistenceConfig represents persistence settings in create request
type CreatePersistenceConfig struct {
	Sessions  *CreateVolumePersistence `json:"sessions,omitempty"`
	Workspace *CreateVolumePersistence `json:"workspace,omitempty"`
}

// CreateVolumePersistence represents volume persistence settings in create request
type CreateVolumePersistence struct {
	StorageClassName string `json:"storageClassName,omitempty"`
	Size             string `json:"size,omitempty"`
}

// TaskResponse represents a task in API responses
type TaskResponse struct {
	Name           string                  `json:"name"`
	Namespace      string                  `json:"namespace"`
	Phase          string                  `json:"phase"`
	Description    string                  `json:"description,omitempty"`
	AgentRef       *AgentReference         `json:"agentRef,omitempty"`
	TemplateRef    *AgentTemplateReference `json:"templateRef,omitempty"`
	PodName        string                  `json:"podName,omitempty"`
	StartTime      *time.Time              `json:"startTime,omitempty"`
	CompletionTime *time.Time              `json:"completionTime,omitempty"`
	Duration       string                  `json:"duration,omitempty"`
	CreatedAt      time.Time               `json:"createdAt"`
	Conditions     []Condition             `json:"conditions,omitempty"`
	Labels         map[string]string       `json:"labels,omitempty"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	TotalCount int  `json:"totalCount"`
	HasMore    bool `json:"hasMore"`
}

// TaskListResponse represents a list of tasks
type TaskListResponse struct {
	Tasks      []TaskResponse `json:"tasks"`
	Total      int            `json:"total"` // Keep for backward compat
	Pagination *Pagination    `json:"pagination,omitempty"`
}

// Condition represents a status condition
type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// CredentialInfo represents credential information (without secrets)
type CredentialInfo struct {
	Name      string `json:"name"`
	SecretRef string `json:"secretRef"`
	MountPath string `json:"mountPath,omitempty"`
	Env       string `json:"env,omitempty"`
}

// QuotaInfo represents quota configuration
type QuotaInfo struct {
	MaxTaskStarts int32 `json:"maxTaskStarts,omitempty"`
	WindowSeconds int32 `json:"windowSeconds,omitempty"`
}

// AgentResponse represents an agent in API responses
type AgentResponse struct {
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	Profile            string            `json:"profile,omitempty"`
	TemplateRef        *AgentReference   `json:"templateRef,omitempty"`
	ExecutorImage      string            `json:"executorImage,omitempty"`
	AgentImage         string            `json:"agentImage,omitempty"`
	WorkspaceDir       string            `json:"workspaceDir,omitempty"`
	ServiceAccountName string            `json:"serviceAccountName,omitempty"`
	ContextsCount      int               `json:"contextsCount"`
	CredentialsCount   int               `json:"credentialsCount"`
	MaxConcurrentTasks *int32            `json:"maxConcurrentTasks,omitempty"`
	Quota              *QuotaInfo        `json:"quota,omitempty"`
	Credentials        []CredentialInfo  `json:"credentials,omitempty"`
	Contexts           []ContextItem     `json:"contexts,omitempty"`
	CreatedAt          time.Time         `json:"createdAt"`
	Labels             map[string]string `json:"labels,omitempty"`
	Standby            *StandbyInfo      `json:"standby,omitempty"`
	Conditions         []Condition       `json:"conditions,omitempty"`
	ServerStatus       *ServerStatusInfo `json:"serverStatus,omitempty"`
}

// AgentTemplateResponse represents an agent template in API responses
type AgentTemplateResponse struct {
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	AgentImage         string            `json:"agentImage,omitempty"`
	ExecutorImage      string            `json:"executorImage,omitempty"`
	WorkspaceDir       string            `json:"workspaceDir,omitempty"`
	ServiceAccountName string            `json:"serviceAccountName,omitempty"`
	ContextsCount      int               `json:"contextsCount"`
	CredentialsCount   int               `json:"credentialsCount"`
	Credentials        []CredentialInfo  `json:"credentials,omitempty"`
	Contexts           []ContextItem     `json:"contexts,omitempty"`
	CreatedAt          time.Time         `json:"createdAt"`
	Labels             map[string]string `json:"labels,omitempty"`
	Conditions         []Condition       `json:"conditions,omitempty"`
	AgentCount         int               `json:"agentCount"`
}

// AgentTemplateListResponse represents a list of agent templates
type AgentTemplateListResponse struct {
	Templates  []AgentTemplateResponse `json:"templates"`
	Total      int                     `json:"total"`
	Pagination *Pagination             `json:"pagination,omitempty"`
}

// StandbyInfo represents standby configuration in API requests/responses
type StandbyInfo struct {
	IdleTimeout string `json:"idleTimeout"`
}

// ServerStatusInfo represents the status of an Agent's deployment
type ServerStatusInfo struct {
	DeploymentName string     `json:"deploymentName,omitempty"`
	ServiceName    string     `json:"serviceName,omitempty"`
	URL            string     `json:"url,omitempty"`
	Ready          bool       `json:"ready"`
	Port           int32      `json:"port,omitempty"`
	Suspended      bool       `json:"suspended"`
	IdleSince      *time.Time `json:"idleSince,omitempty"`
}

// LogEvent represents a Server-Sent Event for log streaming
type LogEvent struct {
	Type     string  `json:"type"`
	Phase    *string `json:"phase,omitempty"`
	PodPhase *string `json:"podPhase,omitempty"`
	Content  *string `json:"content,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// HealthResponse represents the health endpoint response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

// AgentListResponse represents a list of agents
type AgentListResponse struct {
	Agents     []AgentResponse `json:"agents"`
	Total      int             `json:"total"`
	Pagination *Pagination     `json:"pagination,omitempty"`
}

// ConfigResponse represents the KubeOpenCodeConfig in API responses
type ConfigResponse struct {
	Name        string             `json:"name"`
	CreatedAt   time.Time          `json:"createdAt"`
	SystemImage *SystemImageConfig `json:"systemImage,omitempty"`
	Cleanup     *CleanupConfig     `json:"cleanup,omitempty"`
	Proxy       *ProxyConfigInfo   `json:"proxy,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty"`
}

// SystemImageConfig represents system image configuration
type SystemImageConfig struct {
	Image           string `json:"image,omitempty"`
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`
}

// CleanupConfig represents cleanup configuration
type CleanupConfig struct {
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
	MaxRetainedTasks        *int32 `json:"maxRetainedTasks,omitempty"`
}

// ProxyConfigInfo represents proxy configuration in API responses
type ProxyConfigInfo struct {
	HttpProxy  string `json:"httpProxy,omitempty"`
	HttpsProxy string `json:"httpsProxy,omitempty"`
	NoProxy    string `json:"noProxy,omitempty"`
}

// CreateCronTaskRequest represents a request to create a CronTask
type CreateCronTaskRequest struct {
	Name                    string                  `json:"name,omitempty"`
	Schedule                string                  `json:"schedule"`
	TimeZone                string                  `json:"timeZone,omitempty"`
	ConcurrencyPolicy       string                  `json:"concurrencyPolicy,omitempty"`
	Suspend                 bool                    `json:"suspend,omitempty"`
	StartingDeadlineSeconds *int64                  `json:"startingDeadlineSeconds,omitempty"`
	MaxRetainedTasks        *int32                  `json:"maxRetainedTasks,omitempty"`
	Description             string                  `json:"description,omitempty"`
	AgentRef                *AgentReference         `json:"agentRef,omitempty"`
	TemplateRef             *AgentTemplateReference `json:"templateRef,omitempty"`
	Contexts                []ContextItem           `json:"contexts,omitempty"`
}

// UpdateCronTaskRequest represents a request to update a CronTask
type UpdateCronTaskRequest struct {
	Schedule                string  `json:"schedule,omitempty"`
	TimeZone                *string `json:"timeZone,omitempty"`
	ConcurrencyPolicy       string  `json:"concurrencyPolicy,omitempty"`
	Suspend                 *bool   `json:"suspend,omitempty"`
	StartingDeadlineSeconds *int64  `json:"startingDeadlineSeconds,omitempty"`
	MaxRetainedTasks        *int32  `json:"maxRetainedTasks,omitempty"`
	Description             *string `json:"description,omitempty"`
}

// CronTaskResponse represents a CronTask in API responses
type CronTaskResponse struct {
	Name                    string               `json:"name"`
	Namespace               string               `json:"namespace"`
	Schedule                string               `json:"schedule"`
	TimeZone                string               `json:"timeZone,omitempty"`
	ConcurrencyPolicy       string               `json:"concurrencyPolicy"`
	Suspend                 bool                  `json:"suspend"`
	MaxRetainedTasks        int32                 `json:"maxRetainedTasks,omitempty"`
	StartingDeadlineSeconds *int64                `json:"startingDeadlineSeconds,omitempty"`
	Active                  int32                 `json:"active"`
	LastScheduleTime        *time.Time            `json:"lastScheduleTime,omitempty"`
	LastSuccessfulTime      *time.Time            `json:"lastSuccessfulTime,omitempty"`
	NextScheduleTime        *time.Time            `json:"nextScheduleTime,omitempty"`
	TotalExecutions         int64                 `json:"totalExecutions"`
	TaskTemplate            CronTaskTemplateInfo  `json:"taskTemplate"`
	CreatedAt               time.Time             `json:"createdAt"`
	Labels                  map[string]string     `json:"labels,omitempty"`
	Conditions              []Condition           `json:"conditions,omitempty"`
}

// CronTaskTemplateInfo represents the task template in CronTask responses
type CronTaskTemplateInfo struct {
	Description string                  `json:"description,omitempty"`
	AgentRef    *AgentReference         `json:"agentRef,omitempty"`
	TemplateRef *AgentTemplateReference `json:"templateRef,omitempty"`
}

// CronTaskListResponse represents a list of CronTasks
type CronTaskListResponse struct {
	CronTasks  []CronTaskResponse `json:"cronTasks"`
	Total      int                `json:"total"`
	Pagination *Pagination        `json:"pagination,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}
