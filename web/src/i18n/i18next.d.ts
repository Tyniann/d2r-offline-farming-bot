import "i18next";

import { defaultNS, resources } from "./index";

declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: typeof defaultNS;
    returnNull: false;
    resources: (typeof resources)["de"];
  }
}
