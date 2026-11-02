import { FILTER_QUERY_PARAM } from 'src/app/models/event.model';

export type EventSearchType = 'metadata' | 'payload' | 'value' | 'key';

export const EVENT_SEARCH_TYPES: { type: EventSearchType; label: string; placeholder: string; description: string }[] = [
  { type: 'metadata', label: 'Event metadata', placeholder: 'Search event metadata...', description: 'Event ID, event type, source name, or idempotency key.' },
  { type: 'payload', label: 'JSON payload', placeholder: 'Filter payload, e.g. status: "paid"', description: 'Match payload fields and nesting, e.g. status: "paid" or data: {status: "paid"}. Outer braces and quotes around simple field names are optional; string values need double quotes.' },
  { type: 'key', label: 'Idempotency key', placeholder: 'Enter idempotency key...', description: 'The exact stored idempotency key shown in event details.' },
  { type: 'value', label: 'Idempotency value', placeholder: 'Enter idempotency value...', description: 'The exact raw value received at a source’s idempotency key location, such as a provider delivery ID.' }
];

// Saved links predate explicit search types. Match the API's legacy JSON
// promotion only while restoring those links, never while interpreting input.
export function legacyPayloadFilter(query: string): FILTER_QUERY_PARAM | null {
  const start = query.indexOf('{');
  const end = query.lastIndexOf('}');
  if (start < 0 || end <= start) return null;
  try {
    const body = query.slice(start, end + 1);
    const parsed = JSON.parse(body);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed) || !Object.keys(parsed).length) return null;
    return { body, query: (query.slice(0, start) + query.slice(end + 1)).trim() || undefined };
  } catch {
    return null;
  }
}

export function eventSearchFilter(type: EventSearchType, input: string): FILTER_QUERY_PARAM {
  const value = input.trim();
  if (!value) return {};
  switch (type) {
    case 'metadata':
      return { query: value, queryMode: 'text' };
    case 'value':
      return { idempotencyValue: value };
    case 'key':
      return { idempotencyKey: value };
    case 'payload': {
      let parsed: unknown;
      try {
        parsed = JSON.parse(normalizePayloadObject(value));
      } catch {
        throw new Error('Enter payload fields such as status: "paid", with double quotes around string values.');
      }
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed) || !Object.keys(parsed).length) {
        throw new Error('Enter at least one payload field, for example status: "paid".');
      }
      return { body: JSON.stringify(parsed) };
    }
  }
}

function normalizePayloadObject(input: string): string {
  const source = input.startsWith('{') ? input : `{${input}}`;
  let output = '';
  let inString = false;
  let escaped = false;

  for (let index = 0; index < source.length; index++) {
    const char = source[index];
    output += char;
    if (inString) {
      if (escaped) escaped = false;
      else if (char === '\\') escaped = true;
      else if (char === '"') inString = false;
      continue;
    }
    if (char === '"') {
      inString = true;
      continue;
    }
    if (char === '{' || char === ',') {
      const key = source.slice(index + 1).match(/^(\s*)([A-Za-z_$][\w$-]*)(\s*):/);
      if (key) {
        output += key[1] + JSON.stringify(key[2]) + key[3] + ':';
        index += key[0].length;
      }
    }
  }
  return output;
}
