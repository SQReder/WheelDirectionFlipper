package main

import (
	"bufio"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/sys/windows/registry"
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	rootHidPath          = `SYSTEM\CurrentControlSet\Enum\HID`
	KeyNameFlipFlopWheel = "FlipFlopWheel"
)

func listSubKeysNames(key registry.Key) []string {
	stat, err := key.Stat()
	panicOnError(err, "Getting stat of key")

	names, err := key.ReadSubKeyNames(int(stat.SubKeyCount))
	panicOnError(err, "ReadSubKeyNames")

	return names
}

func (d Device) listInstances() []DeviceInstance {
	hidPath := joinPath(rootHidPath, d.id)
	hidKey, err := registry.OpenKey(registry.LOCAL_MACHINE, hidPath, registry.READ)
	panicOnError(err, fmt.Sprintf("HID %s subkey", hidPath))

	hidSubKeys := listSubKeysNames(hidKey)

	var result []DeviceInstance

	for _, hidSubKey := range hidSubKeys {
		subDevice := DeviceInstance{
			parent: d,
			id:     hidSubKey,
		}
		subDevicePath := joinPath(hidPath, hidSubKey)
		subDeviceKey, err := registry.OpenKey(registry.LOCAL_MACHINE, subDevicePath, registry.READ)
		panicOnError(err, "Nya")

		desc, _, err := subDeviceKey.GetStringValue("DeviceDesc")

		subDevice.desc = parseDeviceDesc(desc)

		result = append(result, subDevice)
	}

	return result
}

func parseDeviceDesc(desc string) deviceDesc {
	commaIndex := strings.Index(desc, ",")
	colonIdex := strings.Index(desc, ";")

	driver := desc[:commaIndex]
	name := desc[colonIdex+1:]

	return deviceDesc{
		driver:     driver,
		deviceType: desc[commaIndex+1 : colonIdex],
		name:       name,
	}
}

func (d deviceDesc) isMouse() bool {
	return d.driver == "@msmouse.inf"
}

func listRootDevices() []Device {
	DebugLogger.Printf("Open %s", rootHidPath)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, rootHidPath, registry.READ)
	panicOnError(err, "Open HID key")

	rootNames := listSubKeysNames(key)

	DebugLogger.Printf("Got device names\n\t%v", strings.Join(rootNames, "\n\t"))

	var result []Device

	for _, hidName := range rootNames {
		result = append(result, Device{
			id: hidName,
		})
	}

	return result
}

func (d *DeviceInstance) isFlippable() bool {
	log.Println("Checking subdevice ", d)

	deviceParamsPath := d.getDeviceParamsPath()
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, deviceParamsPath, registry.READ)
	panicOnError(err, "Open Device parameters")

	stat, err := key.Stat()
	panicOnError(err, "Device parameters stat")

	valueCount := stat.ValueCount

	if valueCount == 0 {
		return false
	}

	names, err := key.ReadValueNames(int(valueCount))
	panicOnError(err, "Read value names")

	haveFlipValue := contains(names, "FlipFlopWheel")

	return haveFlipValue
}

func (d *DeviceInstance) getDeviceParamsPath() string {
	return joinPath(rootHidPath, d.parent.id, d.id, "Device Parameters")
}

func (d DeviceInstance) getWheelDirection() int {
	if !d.desc.isMouse() {
		log.Println("Skip", strconv.Quote(d.getFriendlyName()), "- it's not a mouse")
		return WHEEL_UNKNOWN
	}

	defer func() {
		if err := recover(); err != nil {
			log.Println("panic occurred:", err)
		}
	}()

	parametersPath := joinPath(rootHidPath, d.parent.id, d.id, "Device Parameters")
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, parametersPath, registry.READ)
	panicOnError(err, "Open Device parameters")

	stat, err := key.Stat()
	panicOnError(err, "Device Parameters stat")
	valueCount := stat.ValueCount

	if valueCount == 0 {
		return WHEEL_UNKNOWN
	}

	valueNames, err := key.ReadValueNames(int(valueCount))
	if !contains(valueNames, KeyNameFlipFlopWheel) {
		return WHEEL_UNKNOWN
	}

	wheelFlipFlag, _, err := key.GetIntegerValue(KeyNameFlipFlopWheel)
	panicOnError(err, "FlipFlopWheel value read")

	DebugLogger.Println("wheelFlipFlag", wheelFlipFlag)

	return int(wheelFlipFlag)
}

func main() {
	InfoLogger.Println("Starting up")

	rootDevices := listRootDevices()

	configurable := loadDeviceInstances(rootDevices)

	printDevicesTable(configurable)

	fmt.Println("Which device you want to flip?")
	fmt.Print("Please enter the index: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	panicOnError(err, "Read number failed")

	trimmedInput := strings.Trim(string(line), "\n")

	DebugLogger.Println("Line", trimmedInput)

	instanceIndex, err := strconv.Atoi(trimmedInput)
	panicOnError(err, "Convert string to index")

	if instanceIndex < 0 || instanceIndex >= len(configurable) {
		ErrorLogger.Panicf("Unknown index. Expected 0..%d, but receive %d", len(configurable)-1, instanceIndex)
	}

	instance := configurable[instanceIndex]
	toggleWheel(instance)
}

func toggleWheel(instance DeviceInstance) {
	currentDirection := instance.getWheelDirection()

	if currentDirection != WHEEL_FLIPPED && currentDirection != WHEEL_NORMAL {
		ErrorLogger.Panicln("Unknown current direction", instance)
	}

	path := joinPath(rootHidPath, instance.GetDevicePath(), "Device Parameters")
	DebugLogger.Println("Looking for key", path)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.WRITE)
	panicOnError(err, "Open instance for writing")

	flippedValue := currentDirection ^ 1
	DebugLogger.Println("New value is", flippedValue)

	err = key.SetDWordValue("FlipFlopWheel", uint32(flippedValue))
	panicOnError(err, "Set new value")
}

func loadDeviceInstances(devices []Device) []DeviceInstance {
	var mice []DeviceInstance

	for _, device := range devices {
		InfoLogger.Println("Processing device", device.id)
		instances := device.listInstances()
		for _, instance := range instances {
			InfoLogger.Println("Processing instance", instance)

			isMouse := instance.desc.isMouse()

			DebugLogger.Println("\tIs mouse:", instance.desc.isMouse())

			if isMouse {
				mice = append(mice, instance)
			}
		}
	}

	return mice
}

func printDevicesTable(configurable []DeviceInstance) {
	fmt.Println("Configurable devices:")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for i, device := range configurable {
		wheelDirectionString := WheelDirectionToString(device.getWheelDirection())

		var wheelCellFgColor int
		if device.getWheelDirection() == WHEEL_FLIPPED {
			wheelCellFgColor = tablewriter.FgGreenColor
		} else {
			wheelCellFgColor = tablewriter.Normal
		}

		table.Rich(
			[]string{strconv.Itoa(i), device.getFriendlyName(), device.parent.id, device.id, wheelDirectionString},
			[]tablewriter.Colors{{}, {}, {}, {}, {tablewriter.Normal, wheelCellFgColor}})
	}

	table.SetHeader([]string{"Index", "Friendly name", "Device ID", "Device Instance Id", "Wheel direction"})
	table.SetBorder(false)
	table.Render()
}

func (d DeviceInstance) GetDevicePath() string {
	return fmt.Sprintf(`%s\%s`, d.parent.id, d.id)
}

func (d DeviceInstance) getFriendlyName() string {
	return d.desc.name
}

func WheelDirectionToString(dir int) string {
	switch dir {
	case WHEEL_UNKNOWN:
		return "Unknown"
	case WHEEL_NORMAL:
		return "Normal"
	case WHEEL_FLIPPED:
		return "Flipped"
	default:
		panic(fmt.Sprintf("Argument out of range %v", dir))
	}
}
