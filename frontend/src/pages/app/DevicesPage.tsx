import { useCallback, useState } from "react";
import { ConfirmDialog } from "../../components/ConfirmDialog";
import { SectionCard } from "../../components/SectionCard";
import { ErrorState, LoadingState } from "../../components/LoadState";
import { useAsyncData } from "../../hooks/useAsyncData";
import { commandDevice, deleteDevice, listDevices } from "../../services/retrosaveApi";
import type { Device } from "../../services/types";
import { DeviceCompactList } from "./devices/DeviceCompactList";
import { DeviceDetailsModal } from "./devices/DeviceDetailsModal";
import { DeviceFleetSummary } from "./devices/DeviceFleetSummary";
import { commandLabel } from "./devices/helpers";
import type { DeviceCommand } from "./devices/types";

export function DevicesPage(): JSX.Element {
  const loader = useCallback(() => listDevices(), []);
  const { loading, error, data, reload } = useAsyncData(loader, []);
  const [commandKey, setCommandKey] = useState<string | null>(null);
  const [commandMessage, setCommandMessage] = useState<string | null>(null);
  const [commandError, setCommandError] = useState<string | null>(null);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [pendingDeleteDevice, setPendingDeleteDevice] = useState<Device | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);

  async function confirmDeleteDevice(): Promise<void> {
    if (!pendingDeleteDevice) {
      return;
    }
    const device = pendingDeleteDevice;
    setDeleteBusy(true);
    setCommandError(null);
    setCommandMessage(null);
    try {
      await deleteDevice(device.id);
      if (selectedDevice?.id === device.id) {
        setSelectedDevice(null);
      }
      setCommandMessage(`${device.displayName} was removed.`);
      setPendingDeleteDevice(null);
      await reload();
    } catch (err: unknown) {
      setCommandError(err instanceof Error ? err.message : "Device delete failed");
    } finally {
      setDeleteBusy(false);
    }
  }

  async function onCommand(device: Device, command: DeviceCommand): Promise<void> {
    const key = `${device.id}:${command}`;
    setCommandKey(key);
    setCommandMessage(null);
    setCommandError(null);
    try {
      await commandDevice(device.id, command, "devices_page");
      setCommandMessage(`${commandLabel(command)} sent to ${device.displayName}`);
    } catch (err: unknown) {
      setCommandError(err instanceof Error ? err.message : "Command failed");
    } finally {
      setCommandKey(null);
    }
  }

  const devices = data ?? [];

  return (
    <SectionCard
      title="Devices"
      subtitle="A compact helper overview. Open details for full service, config, and folder information."
    >
      {loading ? <LoadingState label="Loading devices..." /> : null}
      {error ? <ErrorState message={error} /> : null}
      {commandError ? <ErrorState message={commandError} /> : null}
      {commandMessage ? <p className="success-state">{commandMessage}</p> : null}
      {data ? (
        <>
          <DeviceFleetSummary devices={devices} />
          <DeviceCompactList
            devices={devices}
            commandKey={commandKey}
            onCommand={onCommand}
            onSelectDevice={setSelectedDevice}
          />
        </>
      ) : null}

      {selectedDevice ? (
        <DeviceDetailsModal
          device={selectedDevice}
          commandKey={commandKey}
          onClose={() => setSelectedDevice(null)}
          onCommand={onCommand}
          onRequestDelete={setPendingDeleteDevice}
        />
      ) : null}

      {pendingDeleteDevice ? (
        <ConfirmDialog
          title="Delete helper device"
          message={`This removes "${pendingDeleteDevice.displayName}" from this server. Existing cloud saves are kept, but this helper must re-enroll before it can sync again.`}
          confirmLabel="Delete device"
          danger
          busy={deleteBusy}
          onConfirm={() => void confirmDeleteDevice()}
          onCancel={() => setPendingDeleteDevice(null)}
        />
      ) : null}
    </SectionCard>
  );
}
