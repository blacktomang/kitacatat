// Shared flat ESLint config for the JS/TS side of the monorepo.
import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist", "**/routeTree.gen.ts"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
);
