package main

type Device struct {
	id string
}

type deviceDesc struct {
	driver     string
	deviceType string
	name       string
}

type DeviceInstance struct {
	id     string
	parent Device
	desc   deviceDesc
}

const (
	WHEEL_UNKNOWN = -1
	WHEEL_NORMAL  = 0
	WHEEL_FLIPPED = 1
)
