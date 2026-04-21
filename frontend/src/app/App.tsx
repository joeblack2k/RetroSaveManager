import { RouterProvider } from "react-router-dom";
import { appRouter } from "./routes";

export function App(): JSX.Element {
  return <RouterProvider router={appRouter} />;
}
