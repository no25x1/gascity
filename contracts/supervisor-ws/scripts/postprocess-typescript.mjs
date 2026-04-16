import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const generatedDir = path.join(__dirname, '..', 'generated', 'typescript');

const SAFE_IDENTIFIER = /^[A-Za-z_$][A-Za-z0-9_$]*$/;

function normalizeQuotedPropertyNames(source) {
  return source.replace(/^(\s*)'([A-Za-z_$][A-Za-z0-9_$]*)'(\??:)/gm, (_, indent, name, suffix) => {
    if (!SAFE_IDENTIFIER.test(name)) {
      return `${indent}'${name}'${suffix}`;
    }
    return `${indent}${name}${suffix}`;
  });
}

function stripAdditionalProperties(source) {
  return source
    .replace(/^\s*additionalProperties\??:\s*Map<string,\s*[^;]+;\n?/gm, '')
    .replace(/^\s*'additionalProperties'\??:\s*Map<string,\s*[^;]+;\n?/gm, '')
    .replace(/\n{3,}/g, '\n\n');
}

async function collectGeneratedFiles() {
  const entries = await fs.readdir(generatedDir, { withFileTypes: true });
  return entries
    .filter((entry) => entry.isFile() && entry.name.endsWith('.ts'))
    .map((entry) => entry.name)
    .sort();
}

async function rewriteFiles(files) {
  for (const file of files) {
    const fullPath = path.join(generatedDir, file);
    let source = await fs.readFile(fullPath, 'utf8');
    source = normalizeQuotedPropertyNames(source);
    source = stripAdditionalProperties(source);
    await fs.writeFile(fullPath, source, 'utf8');
  }
}

async function removeUnreferencedAnonymousSchemas(files) {
  const anonymousFiles = files.filter((file) => /^AnonymousSchema_\d+\.ts$/.test(file));
  if (anonymousFiles.length === 0) {
    return;
  }

  const otherFiles = files.filter((file) => !anonymousFiles.includes(file));
  const contents = new Map();
  for (const file of otherFiles) {
    contents.set(file, await fs.readFile(path.join(generatedDir, file), 'utf8'));
  }

  for (const file of anonymousFiles) {
    const stem = path.basename(file, '.ts');
    const referenced = otherFiles.some((other) => contents.get(other)?.includes(stem));
    if (referenced) {
      throw new Error(`anonymous generated schema ${file} is still referenced`);
    }
    await fs.unlink(path.join(generatedDir, file));
  }
}

async function assertNoGenerationNoise() {
  const files = await collectGeneratedFiles();
  for (const file of files) {
    const fullPath = path.join(generatedDir, file);
    const source = await fs.readFile(fullPath, 'utf8');
    if (source.includes('reservedName')) {
      throw new Error(`reservedName leaked into ${file}`);
    }
    if (source.includes('additionalProperties?:') || source.includes("'additionalProperties'?:")) {
      throw new Error(`additionalProperties leaked into ${file}`);
    }
    if (/AnonymousSchema_\d+/.test(file)) {
      throw new Error(`anonymous schema artifact remains: ${file}`);
    }
  }
}

async function main() {
  const files = await collectGeneratedFiles();
  await rewriteFiles(files);
  await removeUnreferencedAnonymousSchemas(await collectGeneratedFiles());
  await assertNoGenerationNoise();
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err));
  process.exit(1);
});
