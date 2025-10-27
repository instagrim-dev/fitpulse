/// <reference types="vite/client" />

declare interface ImportMetaEnv {
  readonly VITE_ACTIVITY_API_URL?: string;
  readonly VITE_ONTOLOGY_API_URL?: string;
  readonly VITE_DEFAULT_USER_ID?: string;
}

declare interface ImportMeta {
  readonly env: ImportMetaEnv;
}
