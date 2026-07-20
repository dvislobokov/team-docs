// Типы для es-бандла Swagger UI: пакет не поставляет их для этого пути.
declare module "swagger-ui-dist/swagger-ui-es-bundle" {
  const SwaggerUIBundle: (config: Record<string, unknown>) => unknown;
  export default SwaggerUIBundle;
}
declare module "swagger-ui-dist/swagger-ui.css";
