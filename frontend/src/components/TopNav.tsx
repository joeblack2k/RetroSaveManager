import { NavLink } from "react-router-dom";

type TopNavProps = {
  items: Array<{ label: string; to: string }>;
};

export function TopNav({ items }: TopNavProps): JSX.Element {
  return (
    <nav className="top-nav" aria-label="Main">
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          className={({ isActive }) => (isActive ? "top-nav__link top-nav__link--active" : "top-nav__link")}
        >
          {item.label}
        </NavLink>
      ))}
    </nav>
  );
}
