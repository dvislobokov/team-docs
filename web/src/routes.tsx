import { createBrowserRouter } from "react-router-dom";
import { App } from "./App";
import { HomeScreen } from "./components/HomeScreen";
import { PageScreen } from "./components/PageScreen";

// App — layout-роут (сайдбар + палитра), внутри которого рендерятся экраны.
export const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <HomeScreen /> },
      { path: "pages/:id", element: <PageScreen /> },
    ],
  },
]);
