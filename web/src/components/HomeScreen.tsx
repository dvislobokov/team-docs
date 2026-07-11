import { Navigate } from "react-router-dom";
import { useTree } from "../store/tree";
import { EmptyState } from "./EmptyState";
import { Topbar } from "./Topbar";

// Корневой маршрут: пока грузится — ничего; есть страницы — открываем первую;
// пусто — экран «чистого листа».
export function HomeScreen() {
  const { tree, loading } = useTree();

  if (loading) return <Topbar />;
  if (tree.length > 0) return <Navigate to={`/pages/${tree[0].id}`} replace />;

  return (
    <>
      <Topbar />
      <EmptyState />
    </>
  );
}
