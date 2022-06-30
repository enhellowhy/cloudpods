package proxy

import "yunion.io/x/onecloud/pkg/mcclient"

type SessionStatus int
type SessionType int

const (
	SessionStatusInit SessionStatus = iota
	SessionStatusActive
	SessionStatusError
)

const (
	SessionTypeRecordingProxy SessionType = iota
	SessionTypeReplayServer
	SessionTypeProxyPass
)

type VncSession struct {
	Target         string
	TargetHostname string
	InstanceName   string
	TargetPort     string
	Cred           mcclient.TokenCredential
	TargetPassword string
	ID             string
	Status         SessionStatus
	Type           SessionType
	ReplayFilePath string
}
