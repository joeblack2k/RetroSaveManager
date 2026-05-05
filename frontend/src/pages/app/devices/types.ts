import type { Device } from "../../../services/types";

export type DeviceCommand = "sync" | "scan" | "deep_scan";

export type DeviceCommandHandler = (device: Device, command: DeviceCommand) => Promise<void>;
