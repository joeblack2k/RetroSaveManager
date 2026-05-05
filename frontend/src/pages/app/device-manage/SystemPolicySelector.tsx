import type { SystemGroup } from "./options";

type SystemPolicySelectorProps = {
  groups: SystemGroup[];
  allowedSystems: string[];
  onToggleSystem: (slug: string) => void;
  isSystemDisabled: (slug: string) => boolean;
  systemDisabledReason: (slug: string) => string;
};

export function SystemPolicySelector({
  groups,
  allowedSystems,
  onToggleSystem,
  isSystemDisabled,
  systemDisabledReason
}: SystemPolicySelectorProps): JSX.Element {
  return (
    <div className="stack compact">
      {groups.map((group) => (
        <details key={group.manufacturer} className="device-group" open>
          <summary>
            <strong>{group.manufacturer}</strong>
            <span>{group.systems.length} systems</span>
          </summary>
          <div className="device-group__list">
            {group.systems.map((system) => {
              const slug = system.slug ?? "";
              const disabled = isSystemDisabled(slug);
              const reason = systemDisabledReason(slug);
              return (
                <label key={slug} className="sync-option-row">
                  <input
                    type="checkbox"
                    checked={allowedSystems.includes(slug)}
                    disabled={disabled}
                    onChange={() => onToggleSystem(slug)}
                  />
                  <span>
                    {system.name}
                    {disabled ? <small>Blocked: {reason}</small> : null}
                  </span>
                </label>
              );
            })}
          </div>
        </details>
      ))}
    </div>
  );
}
