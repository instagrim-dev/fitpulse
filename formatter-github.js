import fs from 'node:fs';
import path from 'node:path';

const SARIF_SCHEMA = 'https://json.schemastore.org/sarif-2.1.0.json';
const SARIF_PATH = process.env.DCLINT_SARIF_PATH || 'dclint.sarif';
const TOOL_NAME = 'DCLint';
const TOOL_URI = 'https://github.com/zavoloklom/docker-compose-linter';

const annotationLevel = (type = 'warning') => {
  const normalized = type.toLowerCase();
  if (normalized === 'error') return 'error';
  if (normalized === 'warning') return 'warning';
  return 'notice';
};

const sarifLevel = (type = 'warning') => {
  const normalized = type.toLowerCase();
  if (normalized === 'error') return 'error';
  if (normalized === 'warning') return 'warning';
  return 'note';
};

function buildSarif(results = []) {
  const sarifResults = [];
  const ruleMap = new Map();

  results.forEach((result) => {
    const filePath = result.filePath ?? '';
    (result.messages ?? []).forEach((message) => {
      const ruleId = message.rule ?? 'dclint';
      const level = sarifLevel(message.type);
      const line = message.line ?? 1;
      const column = message.column ?? 1;

      sarifResults.push({
        ruleId,
        level,
        message: { text: message.message },
        locations: [
          {
            physicalLocation: {
              artifactLocation: { uri: filePath.replace(/^\.\//, '') },
              region: { startLine: line, startColumn: column }
            }
          }
        ]
      });

      if (!ruleMap.has(ruleId)) {
        ruleMap.set(ruleId, {
          id: ruleId,
          name: ruleId,
          shortDescription: {
            text: message.meta?.description ?? message.message
          },
          helpUri: message.meta?.url,
          properties: {
            tags: [message.category, message.severity].filter(Boolean)
          }
        });
      }
    });
  });

  if (!sarifResults.length) {
    return null;
  }

  return {
    version: '2.1.0',
    $schema: SARIF_SCHEMA,
    runs: [
      {
        tool: {
          driver: {
            name: TOOL_NAME,
            informationUri: TOOL_URI,
            rules: Array.from(ruleMap.values())
          }
        },
        results: sarifResults
      }
    ]
  };
}

function writeSarifReport(sarif) {
  if (!sarif) {
    if (fs.existsSync(SARIF_PATH)) {
      fs.rmSync(SARIF_PATH);
    }
    return;
  }

  const dir = path.dirname(SARIF_PATH);
  if (dir && dir !== '.' && !fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
  fs.writeFileSync(SARIF_PATH, JSON.stringify(sarif, null, 2));
}

function buildAnnotations(results = []) {
  const entries = [];
  results.forEach((result) => {
    const filePath = result.filePath ?? '';
    (result.messages ?? []).forEach((message) => {
      const line = message.line ?? 1;
      const column = message.column ?? 1;
      const level = annotationLevel(message.type);
      const rule = message.rule ?? 'dclint';
      entries.push(
        `::${level} file=${filePath},line=${line},col=${column}::${rule}: ${message.message}`
      );
    });
  });
  return entries.join('\n');
}

export default function formatterGithub(results = []) {
  const sarif = buildSarif(results);
  try {
    writeSarifReport(sarif);
  } catch (error) {
    console.error(`Failed to write SARIF report: ${error.message}`);
  }
  return buildAnnotations(results);
}
