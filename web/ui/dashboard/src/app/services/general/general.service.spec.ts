import { Location } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { GeneralService } from './general.service';

describe('GeneralService payload formatting', () => {
  const service = new GeneralService({} as ActivatedRoute, {} as Location);

  it('preserves valid JSON, including nested and escaped property names', () => {
    const payload = { item_descpion: 'xxx', option_1_1: null, nested: { 'quoted"key': 'value', 'space key': [1, true] } };
    const displayed = service.getCodeSnippetString('event_data', payload);
    expect(JSON.parse(displayed)).toEqual(payload);
    expect(displayed).toContain('"item_descpion":');
  });

  it('preserves false, zero and empty string payloads', () => {
    for (const payload of [false, 0, '']) {
      expect(JSON.parse(service.getCodeSnippetString('event_data', payload))).toEqual(payload);
    }
  });

  it('preserves JSON response bodies and errors without double encoding', () => {
    for (const type of ['res_body', 'error'] as const) {
      expect(JSON.parse(service.getCodeSnippetString(type, '{"nested":{"ok":true}}'))).toEqual({ nested: { ok: true } });
      expect(service.getCodeSnippetString(type, 'plain text')).toBe('plain text');
    }
  });

  it('keeps the missing payload message', () => {
    expect(service.getCodeSnippetString('event_data', undefined)).toBe('No event payload was sent');
  });
});
