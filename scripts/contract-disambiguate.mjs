// Build-pipeline transform (NOT a source edit): oapi-codegen flattens OpenAPI
// component schemas AND parameters into one Go type namespace, so a parameter
// and a schema sharing a name (e.g. ApprovalToken) collide. We run this on the
// DERIVED 3.0 spec to suffix every parameter-component Go type with "Param"
// via x-go-name, leaving the foundation contract (spec/contract/crm.yaml) intact.
import { readFileSync, writeFileSync } from "node:fs";
import { parse, stringify } from "yaml";

const [, , inPath, outPath] = process.argv;
if (!inPath || !outPath) {
  console.error("usage: contract-disambiguate.mjs <in.yaml> <out.yaml>");
  process.exit(1);
}
const doc = parse(readFileSync(inPath, "utf8"));
const params = doc?.components?.parameters ?? {};
let n = 0;
for (const [name, param] of Object.entries(params)) {
  if (param && typeof param === "object" && param["x-go-name"] === undefined) {
    param["x-go-name"] = `${name}Param`;
    n++;
  }
}
writeFileSync(outPath, stringify(doc));
console.log(`disambiguated ${n} parameter component(s) -> *Param`);
