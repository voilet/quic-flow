package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/voilet/quic-flow/pkg/hardware"
)

// ==================== 枚举类型 ====================

// DeployType 部署类型
type DeployType string

const (
	DeployTypeContainer  DeployType = "container"
	DeployTypeKubernetes DeployType = "kubernetes"
	DeployTypeScript     DeployType = "script"
	DeployTypeGitPull    DeployType = "gitpull"
)

// OperationType 操作类型
type OperationType string

const (
	OperationTypeDeploy    OperationType = "deploy"    // 部署 (自动判断 install/update)
	OperationTypeInstall   OperationType = "install"   // 强制安装
	OperationTypeUpdate    OperationType = "update"    // 强制更新
	OperationTypeRollback  OperationType = "rollback"  // 回滚
	OperationTypeUninstall OperationType = "uninstall" // 卸载
)

// ReleaseStatus 发布状态
type ReleaseStatus string

const (
	ReleaseStatusPending    ReleaseStatus = "pending"
	ReleaseStatusScheduled  ReleaseStatus = "scheduled"
	ReleaseStatusApproving  ReleaseStatus = "approving"
	ReleaseStatusRunning    ReleaseStatus = "running"
	ReleaseStatusPaused     ReleaseStatus = "paused"
	ReleaseStatusSuccess    ReleaseStatus = "success"
	ReleaseStatusFailed     ReleaseStatus = "failed"
	ReleaseStatusCancelled  ReleaseStatus = "cancelled"
	ReleaseStatusRollback   ReleaseStatus = "rollback"
)

// TargetType 目标类型
type TargetType string

const (
	TargetTypeHost       TargetType = "host"
	TargetTypeK8sCluster TargetType = "k8s-cluster"
)

// TargetStatus 目标状态
type TargetStatus string

const (
	TargetStatusOnline  TargetStatus = "online"
	TargetStatusOffline TargetStatus = "offline"
	TargetStatusUnknown TargetStatus = "unknown"
)

// VariableType 变量类型
type VariableType string

const (
	VariableTypePlain       VariableType = "plain"
	VariableTypeSecret      VariableType = "secret"
	VariableTypeEnvSpecific VariableType = "env_specific"
	VariableTypeTemplate    VariableType = "template"
)

// StagePhase 阶段类型
type StagePhase string

const (
	StagePhasePreRelease  StagePhase = "pre_release"
	StagePhaseRelease     StagePhase = "release"
	StagePhasePostRelease StagePhase = "post_release"
)

// ErrorAction 错误处理动作
type ErrorAction string

const (
	ErrorActionContinue ErrorAction = "continue"
	ErrorActionStop     ErrorAction = "stop"
	ErrorActionRollback ErrorAction = "rollback"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeHealthCheck  TaskType = "health_check"
	TaskTypeBackup       TaskType = "backup"
	TaskTypeScript       TaskType = "script"
	TaskTypeStopService  TaskType = "stop_service"
	TaskTypeStartService TaskType = "start_service"
	TaskTypeDeploy       TaskType = "deploy"
	TaskTypeRollback     TaskType = "rollback"
	TaskTypeCleanup      TaskType = "cleanup"
	TaskTypeNotify       TaskType = "notify"
	TaskTypeWait         TaskType = "wait"
	TaskTypeApproval     TaskType = "approval"
	TaskTypeCondition    TaskType = "condition"
	TaskTypeDBMigrate    TaskType = "db_migrate"
)

// StrategyType 发布策略类型
type StrategyType string

const (
	StrategyTypeRolling   StrategyType = "rolling"
	StrategyTypeBlueGreen StrategyType = "blue_green"
	StrategyTypeCanary    StrategyType = "canary"
)

// RollbackGranularity 回滚粒度
type RollbackGranularity string

const (
	RollbackGranularityAll    RollbackGranularity = "all"
	RollbackGranularitySingle RollbackGranularity = "single"
)

// ApprovalStatus 审批状态
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

// TargetReleaseStatus 目标发布状态
type TargetReleaseStatus string

const (
	TargetReleaseStatusPending  TargetReleaseStatus = "pending"
	TargetReleaseStatusRunning  TargetReleaseStatus = "running"
	TargetReleaseStatusSuccess  TargetReleaseStatus = "success"
	TargetReleaseStatusFailed   TargetReleaseStatus = "failed"
	TargetReleaseStatusSkipped  TargetReleaseStatus = "skipped"
	TargetReleaseStatusRollback TargetReleaseStatus = "rollback"
)

// InstallStatus 安装状态
type InstallStatus string

const (
	InstallStatusInstalled   InstallStatus = "installed"
	InstallStatusUninstalled InstallStatus = "uninstalled"
	InstallStatusFailed      InstallStatus = "failed"
	InstallStatusUnknown     InstallStatus = "unknown"
)

// ==================== JSON 类型 ====================

// StringMap 字符串映射
type StringMap map[string]string

func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *StringMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, m)
}

// StringSlice 字符串切片
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// ==================== 核心模型 ====================

// Project 项目
type Project struct {
	ID          string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name        string     `gorm:"uniqueIndex;size:100;not null" json:"name"`
	Description string     `gorm:"size:500" json:"description"`
	Type        DeployType `gorm:"size:20;not null" json:"type"`
	RepoURL     string     `gorm:"size:500" json:"repo_url"`
	RepoType    string     `gorm:"size:20" json:"repo_type"` // git/svn

	// 脚本部署配置
	ScriptConfig *ScriptDeployConfig `gorm:"type:jsonb" json:"script_config,omitempty"`

	// 容器部署配置
	ContainerConfig *ContainerDeployConfig `gorm:"type:jsonb" json:"container_config,omitempty"`

	// Kubernetes 部署配置
	KubernetesConfig *KubernetesDeployConfig `gorm:"type:jsonb" json:"kubernetes_config,omitempty"`

	// Git 拉取部署配置
	GitPullConfig *GitPullDeployConfig `gorm:"type:jsonb" json:"gitpull_config,omitempty"`

	// 容器命名配置（用于容器/K8s部署）
	ContainerNaming *ContainerNamingConfig `gorm:"type:jsonb" json:"container_naming,omitempty"`

	// 关联
	Environments []Environment `gorm:"foreignKey:ProjectID" json:"environments,omitempty"`
	Variables    []Variable    `gorm:"foreignKey:ProjectID" json:"variables,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Environment 环境
type Environment struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID   string `gorm:"type:uuid;index;not null" json:"project_id"`
	Name        string `gorm:"size:50;not null" json:"name"` // dev/test/staging/prod
	Description string `gorm:"size:200" json:"description"`

	// 发布窗口
	ReleaseWindow *ReleaseWindow `gorm:"type:jsonb" json:"release_window,omitempty"`

	// 审批配置
	RequireApproval bool        `gorm:"default:false" json:"require_approval"`
	Approvers       StringSlice `gorm:"type:jsonb" json:"approvers,omitempty"`

	// 关联
	Project   Project    `gorm:"foreignKey:ProjectID" json:"-"`
	Targets   []Target   `gorm:"foreignKey:EnvironmentID" json:"targets,omitempty"`
	Variables []Variable `gorm:"foreignKey:EnvironmentID" json:"variables,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ReleaseWindow 发布窗口配置
type ReleaseWindow struct {
	Enabled     bool   `json:"enabled"`
	Timezone    string `json:"timezone"`      // Asia/Shanghai
	AllowedDays []int  `json:"allowed_days"`  // 1-7 (Monday-Sunday)
	StartTime   string `json:"start_time"`    // HH:MM
	EndTime     string `json:"end_time"`      // HH:MM
}

func (r ReleaseWindow) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *ReleaseWindow) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, r)
}

// Target 部署目标
type Target struct {
	ID            string       `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	EnvironmentID string       `gorm:"type:uuid;index;not null" json:"environment_id"`
	ClientID      string       `gorm:"size:100;index;not null" json:"client_id"` // QUIC客户端ID
	Name          string       `gorm:"size:100;not null" json:"name"`
	Type          TargetType   `gorm:"size:20;not null" json:"type"`
	Status        TargetStatus `gorm:"size:20;default:'unknown'" json:"status"`
	Labels        StringMap    `gorm:"type:jsonb" json:"labels,omitempty"`
	Config        TargetConfig `gorm:"type:jsonb" json:"config"`
	Priority      int          `gorm:"default:0" json:"priority"` // 部署优先级，用于金丝雀

	// 关联
	Environment Environment `gorm:"foreignKey:EnvironmentID" json:"-"`

	LastSeenAt *time.Time     `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TargetConfig 目标配置
type TargetConfig struct {
	// Docker配置
	DockerHost    string `json:"docker_host,omitempty"`
	DockerTLSPath string `json:"docker_tls_path,omitempty"`

	// K8s配置
	KubeConfig  string `json:"kubeconfig,omitempty"`
	KubeContext string `json:"kube_context,omitempty"`
	Namespace   string `json:"namespace,omitempty"`

	// 通用配置
	WorkDir string `json:"work_dir,omitempty"`
	User    string `json:"user,omitempty"`
	SSHKey  string `json:"ssh_key,omitempty"`
}

func (t TargetConfig) Value() (driver.Value, error) {
	return json.Marshal(t)
}

func (t *TargetConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, t)
}

// Variable 变量
type Variable struct {
	ID            string       `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID     *string      `gorm:"type:uuid;index" json:"project_id,omitempty"`
	EnvironmentID *string      `gorm:"type:uuid;index" json:"environment_id,omitempty"`
	Name          string       `gorm:"size:100;not null" json:"name"`
	Value         string       `gorm:"type:text" json:"value"`
	Type          VariableType `gorm:"size:20;default:'plain'" json:"type"`
	Description   string       `gorm:"size:200" json:"description"`

	// 环境差异化值
	EnvValues StringMap `gorm:"type:jsonb" json:"env_values,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Release 发布记录
type Release struct {
	ID            string        `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID     string        `gorm:"type:uuid;index;not null" json:"project_id"`
	EnvironmentID string        `gorm:"type:uuid;index;not null" json:"environment_id"`
	Version       string        `gorm:"size:50;not null" json:"version"`
	Operation     OperationType `gorm:"size:20;not null;default:'deploy'" json:"operation"`
	Status        ReleaseStatus `gorm:"size:20;not null;index" json:"status"`

	// 发布配置
	Strategy       ReleaseStrategy `gorm:"type:jsonb" json:"strategy"`
	Variables      StringMap       `gorm:"type:jsonb" json:"variables,omitempty"`
	TargetIDs      StringSlice     `gorm:"type:jsonb" json:"target_ids,omitempty"`
	RollbackConfig *RollbackConfig `gorm:"type:jsonb" json:"rollback_config,omitempty"`

	// 定时发布
	ScheduledAt *time.Time `gorm:"index" json:"scheduled_at,omitempty"`

	// 执行结果
	Results TargetResults `gorm:"type:jsonb" json:"results,omitempty"`

	// 元信息
	CreatedBy  string  `gorm:"size:100;not null" json:"created_by"`
	ApprovedBy *string `gorm:"size:100" json:"approved_by,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// ReleaseStrategy 发布策略
type ReleaseStrategy struct {
	Type StrategyType `json:"type"`

	// 滚动更新配置
	BatchSize     int `json:"batch_size"`
	BatchInterval int `json:"batch_interval"`

	// 金丝雀配置
	CanaryPercent  int         `json:"canary_percent"`
	CanaryTargets  StringSlice `json:"canary_targets,omitempty"`
	VerifyDuration int         `json:"verify_duration"`
	AutoPromote    bool        `json:"auto_promote"`

	// 蓝绿配置
	SwitchTimeout  int  `json:"switch_timeout"`
	KeepOldVersion bool `json:"keep_old_version"`
}

func (r ReleaseStrategy) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *ReleaseStrategy) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, r)
}

// RollbackConfig 回滚配置
type RollbackConfig struct {
	Granularity   RollbackGranularity `json:"granularity"`
	AutoRollback  bool                `json:"auto_rollback"`
	Conditions    []RollbackCondition `json:"conditions,omitempty"`
	TargetVersion string              `json:"target_version,omitempty"`
}

func (r RollbackConfig) Value() (driver.Value, error) {
	return json.Marshal(r)
}

func (r *RollbackConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, r)
}

// RollbackCondition 回滚条件
type RollbackCondition struct {
	Type      string `json:"type"`
	Threshold any    `json:"threshold"`
}

// TargetResults 目标结果列表
type TargetResults []TargetResult

func (t TargetResults) Value() (driver.Value, error) {
	return json.Marshal(t)
}

func (t *TargetResults) Scan(value interface{}) error {
	if value == nil {
		*t = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, t)
}

// TargetResult 目标执行结果
type TargetResult struct {
	TargetID   string              `json:"target_id"`
	TargetName string              `json:"target_name"`
	Status     TargetReleaseStatus `json:"status"`
	StartedAt  *time.Time          `json:"started_at,omitempty"`
	FinishedAt *time.Time          `json:"finished_at,omitempty"`
	Stages     []StageResult       `json:"stages,omitempty"`
	Error      string              `json:"error,omitempty"`
}

// StageResult 阶段执行结果
type StageResult struct {
	Name       string       `json:"name"`
	Phase      StagePhase   `json:"phase"`
	Status     string       `json:"status"`
	Tasks      []TaskResult `json:"tasks,omitempty"`
	StartedAt  *time.Time   `json:"started_at,omitempty"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
}

// TaskResult 任务执行结果
type TaskResult struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       TaskType   `json:"type"`
	Status     string     `json:"status"`
	Output     string     `json:"output,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	RetryCount int        `json:"retry_count"`
}

// ScriptDeployConfig 脚本部署配置
type ScriptDeployConfig struct {
	WorkDir     string    `json:"work_dir"`
	Interpreter string    `json:"interpreter"`
	Environment StringMap `json:"environment,omitempty"`

	// 四种操作脚本
	InstallScript   string `json:"install_script"`
	UpdateScript    string `json:"update_script"`
	RollbackScript  string `json:"rollback_script"`
	UninstallScript string `json:"uninstall_script"`

	// 超时配置
	Timeouts ScriptTimeouts `json:"timeouts"`
}

func (s ScriptDeployConfig) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *ScriptDeployConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// ScriptTimeouts 脚本超时配置
type ScriptTimeouts struct {
	Install   int `json:"install"`
	Update    int `json:"update"`
	Rollback  int `json:"rollback"`
	Uninstall int `json:"uninstall"`
}

// ContainerDeployConfig 容器部署配置（完整版）
type ContainerDeployConfig struct {
	// ==================== 镜像配置 ====================
	Image           string `json:"image"`                         // 镜像地址 (registry/image:tag)
	Registry        string `json:"registry,omitempty"`            // 镜像仓库地址
	RegistryUser    string `json:"registry_user,omitempty"`       // 仓库用户名
	RegistryPass    string `json:"registry_pass,omitempty"`       // 仓库密码 (加密存储)
	ImagePullPolicy string `json:"image_pull_policy,omitempty"`   // always, ifnotpresent, never
	Platform        string `json:"platform,omitempty"`            // 目标平台 (linux/amd64, linux/arm64)

	// ==================== 容器基础配置 ====================
	ContainerName string            `json:"container_name"`            // 容器名称
	Hostname      string            `json:"hostname,omitempty"`        // 主机名
	Domainname    string            `json:"domainname,omitempty"`      // 域名
	User          string            `json:"user,omitempty"`            // 运行用户 (UID:GID 或 username)
	GroupAdd      []string          `json:"group_add,omitempty"`       // 附加用户组
	WorkingDir    string            `json:"working_dir,omitempty"`     // 工作目录
	Environment   map[string]string `json:"environment,omitempty"`     // 环境变量
	Labels        map[string]string `json:"labels,omitempty"`          // 容器标签
	Command       []string          `json:"command,omitempty"`         // 启动命令
	Entrypoint    []string          `json:"entrypoint,omitempty"`      // 入口点

	// ==================== 端口配置 ====================
	Ports       []PortMapping `json:"ports,omitempty"`        // 端口映射
	ExposePorts []int         `json:"expose_ports,omitempty"` // 暴露端口但不映射

	// ==================== 网络配置 ====================
	NetworkMode string   `json:"network_mode,omitempty"` // bridge/host/none/container:name
	Networks    []string `json:"networks,omitempty"`     // 加入的网络
	DNS         []string `json:"dns,omitempty"`          // DNS 服务器
	DNSSearch   []string `json:"dns_search,omitempty"`   // DNS 搜索域
	DNSOpt      []string `json:"dns_opt,omitempty"`      // DNS 选项
	ExtraHosts  []string `json:"extra_hosts,omitempty"`  // 额外主机 (host:ip)
	MacAddress  string   `json:"mac_address,omitempty"`  // MAC 地址
	IPv4Address string   `json:"ipv4_address,omitempty"` // IPv4 地址
	IPv6Address string   `json:"ipv6_address,omitempty"` // IPv6 地址
	Links       []string `json:"links,omitempty"`        // 连接容器 (container:alias)

	// ==================== 存储配置 ====================
	Volumes      []VolumeMount         `json:"volumes,omitempty"`       // 卷挂载
	TmpfsMounts  []TmpfsMount          `json:"tmpfs_mounts,omitempty"`  // tmpfs 挂载
	VolumeDriver string                `json:"volume_driver,omitempty"` // 卷驱动
	StorageOpts  map[string]string     `json:"storage_opts,omitempty"`  // 存储驱动选项

	// ==================== 安全配置 ====================
	Privileged      bool     `json:"privileged,omitempty"`         // 特权模式
	CapAdd          []string `json:"cap_add,omitempty"`            // 添加的 capabilities
	CapDrop         []string `json:"cap_drop,omitempty"`           // 移除的 capabilities
	SecurityOpt     []string `json:"security_opt,omitempty"`       // 安全选项 (apparmor, seccomp)
	ReadOnlyRootfs  bool     `json:"read_only_rootfs,omitempty"`   // 只读根文件系统
	NoNewPrivileges bool     `json:"no_new_privileges,omitempty"`  // 禁止获取新权限
	UsernsMode      string   `json:"userns_mode,omitempty"`        // 用户命名空间模式

	// ==================== 设备配置 ====================
	Devices           []DeviceMapping `json:"devices,omitempty"`             // 设备映射
	GPUs              string          `json:"gpus,omitempty"`                // GPU 配置 (all 或具体 GPU ID)
	DeviceCgroupRules []string        `json:"device_cgroup_rules,omitempty"` // 设备 cgroup 规则

	// ==================== 资源限制 ====================
	MemoryLimit      string   `json:"memory_limit,omitempty"`       // 内存限制 (512m, 1g)
	MemoryReserve    string   `json:"memory_reserve,omitempty"`     // 内存预留
	MemorySwap       string   `json:"memory_swap,omitempty"`        // 内存+交换限制 (-1 无限制)
	MemorySwappiness *int     `json:"memory_swappiness,omitempty"`  // 交换倾向 (0-100)
	CPULimit         string   `json:"cpu_limit,omitempty"`          // CPU 限制 (0.5, 1)
	CPUShares        int      `json:"cpu_shares,omitempty"`         // CPU 权重 (默认 1024)
	CpusetCpus       string   `json:"cpuset_cpus,omitempty"`        // CPU 绑定 (0,1 或 0-3)
	CpusetMems       string   `json:"cpuset_mems,omitempty"`        // 内存节点绑定
	PidsLimit        int64    `json:"pids_limit,omitempty"`         // 进程数限制
	Ulimits          []Ulimit `json:"ulimits,omitempty"`            // ulimit 设置
	OomKillDisable   bool     `json:"oom_kill_disable,omitempty"`   // 禁用 OOM Killer
	OomScoreAdj      int      `json:"oom_score_adj,omitempty"`      // OOM 分数调整 (-1000 到 1000)
	ShmSize          string   `json:"shm_size,omitempty"`           // /dev/shm 大小

	// ==================== 运行时配置 ====================
	Runtime      string            `json:"runtime,omitempty"`       // 运行时 (runc/nvidia)
	Init         bool              `json:"init,omitempty"`          // 使用 tini 作为 init
	PidMode      string            `json:"pid_mode,omitempty"`      // PID 命名空间 (host/container:name)
	IpcMode      string            `json:"ipc_mode,omitempty"`      // IPC 命名空间
	UtsMode      string            `json:"uts_mode,omitempty"`      // UTS 命名空间
	CgroupParent string            `json:"cgroup_parent,omitempty"` // 父 cgroup
	Sysctls      map[string]string `json:"sysctls,omitempty"`       // 内核参数
	StopSignal   string            `json:"stop_signal,omitempty"`   // 停止信号 (默认 SIGTERM)
	Tty          bool              `json:"tty,omitempty"`           // 分配 TTY
	StdinOpen    bool              `json:"stdin_open,omitempty"`    // 保持 stdin 开启

	// ==================== 日志配置 ====================
	LogDriver string            `json:"log_driver,omitempty"` // 日志驱动 (json-file/syslog/journald/none)
	LogOpts   map[string]string `json:"log_opts,omitempty"`   // 日志选项 (max-size, max-file)

	// ==================== 健康检查 ====================
	HealthCheck *ContainerHealthCheck `json:"health_check,omitempty"`

	// ==================== 重启策略 ====================
	RestartPolicy     string `json:"restart_policy,omitempty"`      // no/always/on-failure/unless-stopped
	RestartMaxRetries int    `json:"restart_max_retries,omitempty"` // on-failure 最大重试次数

	// ==================== 部署策略 ====================
	StopTimeout    int  `json:"stop_timeout,omitempty"`    // 停止超时（秒）
	RemoveOld      bool `json:"remove_old,omitempty"`      // 移除旧容器
	KeepOldCount   int  `json:"keep_old_count,omitempty"`  // 保留旧容器数量
	PullBeforeStop bool `json:"pull_before_stop,omitempty"` // 先拉取镜像再停止
	AutoRemove     bool `json:"auto_remove,omitempty"`     // 退出时自动删除容器

	// ==================== 默认资源限制（可被版本/任务覆盖） ====================
	DefaultResources *ResourceLimits `json:"default_resources,omitempty"`

	// ==================== 部署脚本（默认，可被版本覆盖） ====================
	PreScript  string `json:"pre_script,omitempty"`  // 部署前脚本
	PostScript string `json:"post_script,omitempty"` // 部署后脚本
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol,omitempty"` // tcp, udp
	HostIP        string `json:"host_ip,omitempty"`  // 绑定的主机IP
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
	Type          string `json:"type,omitempty"` // bind, volume, tmpfs
}

// TmpfsMount tmpfs 挂载
type TmpfsMount struct {
	ContainerPath string `json:"container_path"`        // 容器内挂载路径
	Size          string `json:"size,omitempty"`        // 大小限制 (如 64m, 1g)
	Mode          int    `json:"mode,omitempty"`        // 权限模式 (如 1777)
}

// DeviceMapping 设备映射
type DeviceMapping struct {
	HostPath      string `json:"host_path"`              // 主机设备路径
	ContainerPath string `json:"container_path"`         // 容器设备路径
	Permissions   string `json:"permissions,omitempty"`  // 权限 (rwm)
}

// Ulimit 资源限制
type Ulimit struct {
	Name string `json:"name"`  // 资源名称 (nofile, nproc, core, etc.)
	Soft int64  `json:"soft"`  // 软限制
	Hard int64  `json:"hard"`  // 硬限制
}

// ContainerHealthCheck 容器健康检查
type ContainerHealthCheck struct {
	Command     []string `json:"command"`               // 检查命令
	Interval    int      `json:"interval,omitempty"`    // 检查间隔（秒）
	Timeout     int      `json:"timeout,omitempty"`     // 超时时间（秒）
	Retries     int      `json:"retries,omitempty"`     // 重试次数
	StartPeriod int      `json:"start_period,omitempty"` // 启动等待期（秒）
}

// ==================== 配置分层设计 ====================

// ResourceLimits 资源限制（可被版本/任务覆盖）
type ResourceLimits struct {
	CPURequest    string `json:"cpu_request,omitempty"`    // CPU 请求 (100m, 0.5)
	CPULimit      string `json:"cpu_limit,omitempty"`      // CPU 限制 (500m, 1)
	MemoryRequest string `json:"memory_request,omitempty"` // 内存请求 (128Mi, 256m)
	MemoryLimit   string `json:"memory_limit,omitempty"`   // 内存限制 (512Mi, 1g)
}

// VersionDeployConfig 版本部署配置（发布单元）
// 用于定义"运行什么"，每次发布创建新版本时配置
type VersionDeployConfig struct {
	// ========== 镜像配置 ==========
	Image string `json:"image"` // 镜像地址或 tag（必填）

	// ========== 环境变量（增量合并到项目配置） ==========
	Environment map[string]string `json:"environment,omitempty"`

	// ========== 资源限制覆盖 ==========
	Resources *ResourceLimits `json:"resources,omitempty"`

	// ========== K8s 副本数覆盖 ==========
	Replicas *int `json:"replicas,omitempty"`

	// ========== 启动命令覆盖 ==========
	Command    []string `json:"command,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty"`
	WorkingDir string   `json:"working_dir,omitempty"`

	// ========== 健康检查覆盖（完全替换项目配置） ==========
	HealthCheck *ContainerHealthCheck `json:"health_check,omitempty"`

	// ========== 部署脚本（版本特定） ==========
	PreScript  string `json:"pre_script,omitempty"`  // 部署前脚本
	PostScript string `json:"post_script,omitempty"` // 部署后脚本

	// ========== K8s YAML 覆盖（高级） ==========
	K8sYAMLPatch string `json:"k8s_yaml_patch,omitempty"` // YAML patch
	K8sYAMLFull  string `json:"k8s_yaml_full,omitempty"`  // 完整 YAML（忽略其他配置）
}

func (v VersionDeployConfig) Value() (driver.Value, error) {
	return json.Marshal(v)
}

func (v *VersionDeployConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, v)
}

// TaskOverrideConfig 任务覆盖配置（执行覆盖）
// 用于临时调整，如金丝雀、A/B 测试、紧急扩容等场景
type TaskOverrideConfig struct {
	// ========== 镜像覆盖（临时） ==========
	Image string `json:"image,omitempty"` // 用于测试未发布的镜像

	// ========== 环境变量追加（临时） ==========
	EnvironmentAdd map[string]string `json:"environment_add,omitempty"` // 追加到版本环境变量之上

	// ========== 资源覆盖（临时） ==========
	Resources *ResourceLimits `json:"resources,omitempty"` // 紧急扩容或金丝雀使用不同资源

	// ========== 副本数覆盖（临时） ==========
	Replicas *int `json:"replicas,omitempty"` // 紧急扩缩容

	// ========== 启动命令覆盖（临时，调试用） ==========
	Command []string `json:"command,omitempty"`
}

func (t TaskOverrideConfig) Value() (driver.Value, error) {
	return json.Marshal(t)
}

func (t *TaskOverrideConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, t)
}

func (c ContainerDeployConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *ContainerDeployConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, c)
}

// GitPullDeployConfig Git 拉取部署配置
type GitPullDeployConfig struct {
	// Git 仓库配置
	RepoURL    string `json:"repo_url"`              // 仓库地址
	Branch     string `json:"branch,omitempty"`      // 分支，默认 main
	Tag        string `json:"tag,omitempty"`         // 标签（优先于分支）
	Commit     string `json:"commit,omitempty"`      // 指定 commit
	Depth      int    `json:"depth,omitempty"`       // 克隆深度，0 表示完整克隆
	Submodules bool   `json:"submodules,omitempty"`  // 初始化子模块

	// 认证配置
	AuthType   string `json:"auth_type,omitempty"`   // none, ssh, token, basic
	SSHKey     string `json:"ssh_key,omitempty"`     // SSH 私钥
	Token      string `json:"token,omitempty"`       // Access Token
	Username   string `json:"username,omitempty"`    // 用户名
	Password   string `json:"password,omitempty"`    // 密码

	// 部署配置
	WorkDir       string `json:"work_dir"`                 // 工作目录
	CleanBefore   bool   `json:"clean_before,omitempty"`   // 部署前清理
	BackupBefore  bool   `json:"backup_before,omitempty"`  // 部署前备份
	BackupDir     string `json:"backup_dir,omitempty"`     // 备份目录
	BackupKeep    int    `json:"backup_keep,omitempty"`    // 保留备份数量

	// 部署脚本
	PreScript     string            `json:"pre_script,omitempty"`     // 部署前执行的脚本
	PostScript    string            `json:"post_script,omitempty"`    // 部署后执行的脚本
	Environment   map[string]string `json:"environment,omitempty"`    // 环境变量
	Interpreter   string            `json:"interpreter,omitempty"`    // 脚本解释器

	// 超时配置
	CloneTimeout  int `json:"clone_timeout,omitempty"`  // 克隆超时（秒）
	ScriptTimeout int `json:"script_timeout,omitempty"` // 脚本执行超时（秒）
}

func (g GitPullDeployConfig) Value() (driver.Value, error) {
	return json.Marshal(g)
}

func (g *GitPullDeployConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, g)
}

// KubernetesDeployConfig Kubernetes 部署配置
type KubernetesDeployConfig struct {
	// 基础配置
	Namespace     string `json:"namespace,omitempty"`      // 命名空间
	ResourceType  string `json:"resource_type,omitempty"`  // deployment/statefulset/daemonset
	ResourceName  string `json:"resource_name,omitempty"`  // 资源名称
	ContainerName string `json:"container_name,omitempty"` // 容器名称

	// YAML 配置
	YAML         string `json:"yaml,omitempty"`          // 完整的 K8s YAML
	YAMLTemplate string `json:"yaml_template,omitempty"` // YAML 模板

	// 镜像配置
	Image           string `json:"image,omitempty"`
	Registry        string `json:"registry,omitempty"`
	RegistryUser    string `json:"registry_user,omitempty"`
	RegistryPass    string `json:"registry_pass,omitempty"`
	ImagePullPolicy string `json:"image_pull_policy,omitempty"`
	ImagePullSecret string `json:"image_pull_secret,omitempty"`

	// 副本配置
	Replicas    int  `json:"replicas,omitempty"`
	AutoScaling bool `json:"auto_scaling,omitempty"`

	// 更新策略
	UpdateStrategy     string `json:"update_strategy,omitempty"`
	MaxUnavailable     string `json:"max_unavailable,omitempty"`
	MaxSurge           string `json:"max_surge,omitempty"`
	MinReadySeconds    int    `json:"min_ready_seconds,omitempty"`
	RevisionHistoryLim int    `json:"revision_history_limit,omitempty"`

	// 资源配置
	CPURequest    string `json:"cpu_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`

	// 环境变量
	Environment map[string]string `json:"environment,omitempty"`

	// 服务端口
	ServiceType  string    `json:"service_type,omitempty"`
	ServicePorts []K8sPort `json:"service_ports,omitempty"`

	// 部署超时
	DeployTimeout  int `json:"deploy_timeout,omitempty"`
	RolloutTimeout int `json:"rollout_timeout,omitempty"`

	// kubeconfig
	KubeConfig  string `json:"kubeconfig,omitempty"`
	KubeContext string `json:"kube_context,omitempty"`

	// 默认配置（可被版本/任务覆盖）
	DefaultReplicas  int             `json:"default_replicas,omitempty"`  // 默认副本数
	DefaultResources *ResourceLimits `json:"default_resources,omitempty"` // 默认资源限制

	// 部署脚本（默认，可被版本覆盖）
	PreScript  string `json:"pre_script,omitempty"`  // 部署前脚本
	PostScript string `json:"post_script,omitempty"` // 部署后脚本
}

// K8sPort K8s 端口配置
type K8sPort struct {
	Name       string `json:"name,omitempty"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	NodePort   int    `json:"node_port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

func (k KubernetesDeployConfig) Value() (driver.Value, error) {
	return json.Marshal(k)
}

func (k *KubernetesDeployConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, k)
}

// Version 版本（每个项目可创建多个版本）
type Version struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID   string `gorm:"type:uuid;index;not null" json:"project_id"`
	Version     string `gorm:"size:50;not null" json:"version"`
	Description string `gorm:"size:500" json:"description"`
	WorkDir     string `gorm:"size:500;default:'/opt/app'" json:"work_dir"`

	// 四种操作脚本
	InstallScript   string `gorm:"type:text" json:"install_script"`
	UpdateScript    string `gorm:"type:text" json:"update_script"`
	RollbackScript  string `gorm:"type:text" json:"rollback_script"`
	UninstallScript string `gorm:"type:text" json:"uninstall_script"`

	// Git 相关字段（用于 gitpull 类型项目）
	GitRef     string `gorm:"size:200" json:"git_ref"`      // tag/branch/commit 值
	GitRefType string `gorm:"size:20" json:"git_ref_type"`  // tag/branch/commit 类型

	// 容器相关字段（用于 container/kubernetes 类型项目）- 保留用于向后兼容
	ContainerImage string `gorm:"size:500" json:"container_image"` // 镜像地址
	ContainerEnv   string `gorm:"type:text" json:"container_env"`  // 环境变量（KEY=value 格式）
	Replicas       int    `gorm:"default:1" json:"replicas"`       // K8s 副本数
	K8sYAML        string `gorm:"type:text" json:"k8s_yaml"`       // K8s YAML 配置

	// 新版部署配置（统一的版本配置，推荐使用）
	DeployConfig *VersionDeployConfig `gorm:"type:jsonb" json:"deploy_config,omitempty"`

	// 进程监控配置（部署后自动采集进程信息）
	ProcessConfig *ProcessMonitorConfig `gorm:"type:jsonb" json:"process_config,omitempty"`

	// 状态
	Status string `gorm:"size:20;default:'draft'" json:"status"` // draft, active, deprecated

	// 统计
	DeployCount int `gorm:"default:0" json:"deploy_count"`

	// 关联
	Project Project `gorm:"foreignKey:ProjectID" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// VersionStatus 版本状态
type VersionStatus string

const (
	VersionStatusDraft      VersionStatus = "draft"
	VersionStatusActive     VersionStatus = "active"
	VersionStatusDeprecated VersionStatus = "deprecated"
)

// DeployTask 部署任务
type DeployTask struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID string `gorm:"type:uuid;index;not null" json:"project_id"`
	VersionID string `gorm:"type:uuid;index;not null" json:"version_id"`
	Version   string `gorm:"size:50;not null" json:"version"` // 冗余存储版本号

	// 操作类型
	Operation OperationType `gorm:"size:20;not null" json:"operation"`

	// 目标
	ClientIDs StringSlice `gorm:"type:jsonb" json:"client_ids"`

	// 版本升级自动选择客户端配置
	SourceVersion     string `gorm:"size:50" json:"source_version,omitempty"`     // 源版本过滤，用于升级场景
	AutoSelectClients bool   `gorm:"default:false" json:"auto_select_clients"`    // 自动选择已安装客户端
	SelectedFromVersion string `gorm:"size:50" json:"selected_from_version,omitempty"` // 记录来源版本

	// 执行计划
	ScheduleType string     `gorm:"size:20;default:'immediate'" json:"schedule_type"` // immediate, scheduled
	ScheduleFrom *time.Time `json:"schedule_from,omitempty"`
	ScheduleTo   *time.Time `json:"schedule_to,omitempty"`

	// 金丝雀配置
	CanaryEnabled     bool `gorm:"default:false" json:"canary_enabled"`
	CanaryPercent     int  `gorm:"default:10" json:"canary_percent"`
	CanaryDuration    int  `gorm:"default:30" json:"canary_duration"` // 观察时间（分钟）
	CanaryAutoPromote bool `gorm:"default:false" json:"canary_auto_promote"`

	// 失败处理
	FailureStrategy string `gorm:"size:20;default:'continue'" json:"failure_strategy"` // continue, pause, abort
	AutoRollback    bool   `gorm:"default:true" json:"auto_rollback"`                  // 升级失败自动回滚

	// 任务覆盖配置（用于金丝雀、A/B测试、紧急扩容等场景）
	OverrideConfig *TaskOverrideConfig `gorm:"type:jsonb" json:"override_config,omitempty"`

	// 回调通知配置
	CallbackConfigID string      `gorm:"type:uuid;index" json:"callback_config_id,omitempty"`
	CallbackEvents   StringSlice `gorm:"type:jsonb" json:"callback_events,omitempty"` // canary_started, canary_completed, full_completed

	// 任务状态
	Status       string `gorm:"size:20;default:'pending';index" json:"status"` // pending, scheduled, running, canary, paused, completed, failed, cancelled
	TotalCount   int    `gorm:"default:0" json:"total_count"`
	SuccessCount int    `gorm:"default:0" json:"success_count"`
	FailedCount  int    `gorm:"default:0" json:"failed_count"`
	PendingCount int    `gorm:"default:0" json:"pending_count"`

	// 执行结果
	Results DeployTaskResults `gorm:"type:jsonb" json:"results,omitempty"`

	// 创建者
	CreatedBy string `gorm:"size:100;not null" json:"created_by"`

	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// DeployTaskStatus 部署任务状态
type DeployTaskStatus string

const (
	DeployTaskStatusPending   DeployTaskStatus = "pending"
	DeployTaskStatusScheduled DeployTaskStatus = "scheduled"
	DeployTaskStatusRunning   DeployTaskStatus = "running"
	DeployTaskStatusCanary    DeployTaskStatus = "canary"
	DeployTaskStatusPaused    DeployTaskStatus = "paused"
	DeployTaskStatusCompleted DeployTaskStatus = "completed"
	DeployTaskStatusFailed    DeployTaskStatus = "failed"
	DeployTaskStatusCancelled DeployTaskStatus = "cancelled"
)

// DeployTaskResults 部署任务结果列表
type DeployTaskResults []DeployTaskResult

func (t DeployTaskResults) Value() (driver.Value, error) {
	return json.Marshal(t)
}

func (t *DeployTaskResults) Scan(value interface{}) error {
	if value == nil {
		*t = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, t)
}

// DeployTaskResult 单个目标的执行结果
type DeployTaskResult struct {
	ClientID    string     `json:"client_id"`
	Status      string     `json:"status"` // pending, running, success, failed, skipped
	IsCanary    bool       `json:"is_canary"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Duration    int        `json:"duration"` // 秒
	Error       string     `json:"error,omitempty"`
	Output      string     `json:"output,omitempty"`
	ContainerID string     `json:"container_id,omitempty"` // 容器部署时的 Container ID
}

// TargetInstallation 目标安装状态
type TargetInstallation struct {
	ID            string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	TargetID      string    `gorm:"type:uuid;index;not null" json:"target_id"`
	ProjectID     string    `gorm:"type:uuid;index;not null" json:"project_id"`
	Version       string    `gorm:"size:50;not null" json:"version"`
	Status        string    `gorm:"size:20;not null" json:"status"`
	InstalledAt   time.Time `gorm:"not null" json:"installed_at"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`

	// 备份信息
	BackupPath  string `gorm:"size:500" json:"backup_path,omitempty"`
	BackupCount int    `gorm:"default:0" json:"backup_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatusReport 状态上报记录
type StatusReport struct {
	ID        string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ReleaseID string     `gorm:"type:uuid;index;not null" json:"release_id"`
	TargetID  string     `gorm:"type:uuid;index;not null" json:"target_id"`
	ClientID  string     `gorm:"size:100;index;not null" json:"client_id"`
	Phase     StagePhase `gorm:"size:20;not null" json:"phase"`
	TaskID    string     `gorm:"size:50" json:"task_id,omitempty"`
	TaskName  string     `gorm:"size:100" json:"task_name,omitempty"`
	Status    string     `gorm:"size:20;not null" json:"status"`
	Progress  int        `gorm:"default:0" json:"progress"`
	Message   string     `gorm:"type:text" json:"message,omitempty"`
	Metrics   StringMap  `gorm:"type:jsonb" json:"metrics,omitempty"`

	ReportedAt time.Time `gorm:"index;not null" json:"reported_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (StatusReport) TableName() string {
	return "release_status_reports"
}

// Approval 审批记录
type Approval struct {
	ID         string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ReleaseID  string         `gorm:"type:uuid;index;not null" json:"release_id"`
	Status     ApprovalStatus `gorm:"size:20;not null;index" json:"status"`
	Approvers  StringSlice    `gorm:"type:jsonb" json:"approvers,omitempty"`
	ApprovedBy *string        `gorm:"size:100" json:"approved_by,omitempty"`
	Comment    string         `gorm:"type:text" json:"comment,omitempty"`

	ExpireAt  time.Time `gorm:"index" json:"expire_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Approval) TableName() string {
	return "release_approvals"
}

// ServiceDependency 服务依赖
type ServiceDependency struct {
	ID          string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ProjectID   string `gorm:"type:uuid;index;not null" json:"project_id"`
	ServiceID   string `gorm:"type:uuid;index;not null" json:"service_id"`
	DependsOnID string `gorm:"type:uuid;index;not null" json:"depends_on_id"`
	Required    bool   `gorm:"default:true" json:"required"`
	WaitReady   bool   `gorm:"default:true" json:"wait_ready"`
	Timeout     int    `gorm:"default:300" json:"timeout"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ServiceDependency) TableName() string {
	return "release_service_dependencies"
}

// DeployLog 部署日志（记录每次部署的详细信息）
type DeployLog struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	TaskID    string `gorm:"type:uuid;index;not null" json:"task_id"`
	ProjectID string `gorm:"type:uuid;index;not null" json:"project_id"`
	VersionID string `gorm:"type:uuid;index" json:"version_id"`
	Version   string `gorm:"size:50;not null" json:"version"`
	ClientID  string `gorm:"size:100;index;not null" json:"client_id"`

	// 操作信息
	Operation OperationType `gorm:"size:20;not null" json:"operation"`
	IsCanary  bool          `gorm:"default:false" json:"is_canary"`

	// 执行结果
	Status      string `gorm:"size:20;not null;index" json:"status"` // success, failed, skipped, rollback
	ExitCode    int    `gorm:"default:0" json:"exit_code"`
	Output      string `gorm:"type:text" json:"output,omitempty"`
	Error       string `gorm:"type:text" json:"error,omitempty"`
	ContainerID string `gorm:"size:100;index" json:"container_id,omitempty"` // 容器部署时的 Container ID

	// 时间信息
	StartedAt  time.Time `gorm:"not null" json:"started_at"`
	FinishedAt time.Time `gorm:"not null" json:"finished_at"`
	Duration   int       `gorm:"default:0" json:"duration"` // 秒

	// 执行者
	CreatedBy string `gorm:"size:100" json:"created_by"`

	CreatedAt time.Time `json:"created_at"`
}

func (DeployLog) TableName() string {
	return "deploy_logs"
}

// DeployLogStatus 部署日志状态
type DeployLogStatus string

const (
	DeployLogStatusSuccess  DeployLogStatus = "success"
	DeployLogStatusFailed   DeployLogStatus = "failed"
	DeployLogStatusSkipped  DeployLogStatus = "skipped"
	DeployLogStatusRollback DeployLogStatus = "rollback"
)

// DeployStats 部署统计
type DeployStats struct {
	TotalCount   int64   `json:"total_count"`
	SuccessCount int64   `json:"success_count"`
	FailedCount  int64   `json:"failed_count"`
	SuccessRate  float64 `json:"success_rate"`
}

// ==================== 版本升级增强模型 ====================

// InstallationInfo 已安装目标信息（API 响应）
type InstallationInfo struct {
	ClientID      string    `json:"client_id"`
	TargetID      string    `json:"target_id"`
	TargetName    string    `json:"target_name"`
	Environment   string    `json:"environment"`
	EnvironmentID string    `json:"environment_id"`
	Version       string    `json:"version"`
	Status        string    `json:"status"`
	InstalledAt   time.Time `json:"installed_at"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

// ContainerNamingConfig 容器命名配置
type ContainerNamingConfig struct {
	Prefix     string `json:"prefix"`               // 前缀，如 "myapp"
	Separator  string `json:"separator,omitempty"`  // 分隔符，默认 "-"
	IncludeEnv bool   `json:"include_env"`          // 包含环境名
	IncludeVer bool   `json:"include_ver"`          // 包含版本号
	MaxLength  int    `json:"max_length,omitempty"` // 最大长度，默认 63

	// 容器名称模板，支持变量: ${PREFIX}, ${ENV}, ${VERSION}, ${TIMESTAMP}, ${INDEX}
	Template string `json:"template,omitempty"`
}

func (c ContainerNamingConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *ContainerNamingConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, c)
}

// ==================== 进程监控模型 ====================

// ProcessMonitorConfig 进程监控配置
type ProcessMonitorConfig struct {
	Rules           []ProcessMatchRule `json:"rules"`                      // 进程匹配规则
	CollectInterval int                `json:"collect_interval,omitempty"` // 采集间隔（秒），默认 60
	CollectResources bool              `json:"collect_resources"`          // 采集资源占用
	AlertOnExit     bool               `json:"alert_on_exit"`              // 进程退出告警
	AlertOnHighCPU  int                `json:"alert_on_high_cpu,omitempty"` // CPU 使用率告警阈值（%）
	AlertOnHighMem  int                `json:"alert_on_high_mem,omitempty"` // 内存使用率告警阈值（%）
}

func (p ProcessMonitorConfig) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *ProcessMonitorConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, p)
}

// ProcessMatchRule 进程匹配规则
type ProcessMatchRule struct {
	Type    string `json:"type"`    // name, cmdline, pidfile, port
	Pattern string `json:"pattern"` // 匹配模式
	Name    string `json:"name"`    // 显示名称
}

// ProcessReport 进程上报记录
type ProcessReport struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ClientID  string    `gorm:"size:100;index;not null" json:"client_id"`
	ProjectID string    `gorm:"type:uuid;index;not null" json:"project_id"`
	VersionID string    `gorm:"type:uuid;index" json:"version_id,omitempty"`
	Version   string    `gorm:"size:50" json:"version,omitempty"`
	ReleaseID string    `gorm:"type:uuid;index" json:"release_id,omitempty"`

	// 进程信息
	Processes ProcessInfoList `gorm:"type:jsonb" json:"processes"`

	ReportedAt time.Time `gorm:"index;not null" json:"reported_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ProcessReport) TableName() string {
	return "process_reports"
}

// ProcessInfoList 进程信息列表
type ProcessInfoList []ProcessInfo

func (p ProcessInfoList) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *ProcessInfoList) Scan(value interface{}) error {
	if value == nil {
		*p = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, p)
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID        int       `json:"pid"`
	Name       string    `json:"name"`
	Cmdline    string    `json:"cmdline"`
	StartTime  time.Time `json:"start_time"`
	Status     string    `json:"status"`      // running, sleeping, zombie
	CPUPercent float64   `json:"cpu_percent"`
	MemoryMB   float64   `json:"memory_mb"`
	MemoryPct  float64   `json:"memory_pct"`
	MatchedBy  string    `json:"matched_by"`  // 匹配规则名称
}

// ==================== 容器上报模型 ====================

// ContainerReport 容器上报记录
type ContainerReport struct {
	ID        string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ClientID  string `gorm:"size:100;index;not null" json:"client_id"`
	ProjectID string `gorm:"type:uuid;index" json:"project_id,omitempty"`

	// 容器信息
	Containers ContainerInfoList `gorm:"type:jsonb" json:"containers"`

	// 采集信息
	DockerVersion string `gorm:"size:50" json:"docker_version,omitempty"`
	TotalCount    int    `gorm:"default:0" json:"total_count"`
	RunningCount  int    `gorm:"default:0" json:"running_count"`

	ReportedAt time.Time `gorm:"index;not null" json:"reported_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ContainerReport) TableName() string {
	return "container_reports"
}

// ContainerInfoList 容器信息列表
type ContainerInfoList []ContainerInfo

func (c ContainerInfoList) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *ContainerInfoList) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, c)
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	Image         string    `json:"image"`
	Status        string    `json:"status"`     // running, exited, paused
	State         string    `json:"state"`      // created, running, paused, restarting, removing, exited, dead
	CreatedAt     time.Time `json:"created_at"`
	StartedAt     time.Time `json:"started_at"`

	// 资源占用
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   int64   `json:"memory_usage"`   // bytes
	MemoryLimit   int64   `json:"memory_limit"`   // bytes
	MemoryPercent float64 `json:"memory_percent"`

	// 网络
	NetworkRx int64 `json:"network_rx"` // bytes
	NetworkTx int64 `json:"network_tx"` // bytes

	// 项目归属（按前缀匹配）
	MatchedProject string `json:"matched_project,omitempty"`
	MatchedPrefix  string `json:"matched_prefix,omitempty"`
}

// ==================== 实时部署日志模型 ====================

// DeployTaskLog 部署任务执行日志（用于实时日志流）
type DeployTaskLog struct {
	ID       string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	TaskID   string `gorm:"type:uuid;index;not null" json:"task_id"`
	ClientID string `gorm:"size:100;index;not null" json:"client_id"`

	// 日志内容
	Level   string `gorm:"size:20;default:'info'" json:"level"` // info, warn, error, debug
	Stage   string `gorm:"size:50" json:"stage,omitempty"`      // pull, stop, remove, run, health_check, script, git_clone, etc.
	Message string `gorm:"type:text;not null" json:"message"`

	// 时间戳
	Timestamp time.Time `gorm:"index;not null" json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}

func (DeployTaskLog) TableName() string {
	return "deploy_task_logs"
}

// ==================== 数据库迁移 ====================

// AllModels 所有模型列表
var AllModels = []interface{}{
	&Project{},
	&Environment{},
	&Target{},
	&Variable{},
	&Version{},
	&DeployTask{},
	&DeployLog{},
	&DeployTaskLog{},
	&Release{},
	&TargetInstallation{},
	&StatusReport{},
	&Approval{},
	&ServiceDependency{},
	&ProcessReport{},
	&ContainerReport{},
	// 回调功能模型
	&CallbackConfig{},
	&CallbackHistory{},
	&MessageTemplate{},
	// 硬件信息模型（简化版，不包含 device_disks 和 device_nics）
	&hardware.Device{},
	&hardware.DeviceHardwareHistory{},
	// 凭证管理模型
	&Credential{},
	&ProjectCredential{},
	// Webhook 触发器模型
	&WebhookConfig{},
	&TriggerRecord{},
	// 成员权限模型
	&User{},
	&ProjectMember{},
	// 配置中心模型
	// 注意：配置中心模型在 config.Models 中定义，需要在迁移时单独调用
}
