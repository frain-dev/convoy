import { eventSearchFilter } from './event-log-search';

describe('Explicit event search types', () => {
  it('keeps JSON-looking metadata as literal text', () => {
    const value = 'invoice.paid {"status":"paid"}';
    expect(eventSearchFilter('metadata', value)).toEqual({ query: value, queryMode: 'text' });
  });

  it('sends payload containment only through body', () => {
    expect(eventSearchFilter('payload', '{ "data": { "status": "paid" } }')).toEqual({ body: '{"data":{"status":"paid"}}' });
  });

  it('accepts field fragments and unquoted keys only in payload mode', () => {
    for (const value of ['status: "paid"', '"status": "paid"', '{status:"paid"}']) {
      expect(eventSearchFilter('payload', value)).toEqual({ body: '{"status":"paid"}' });
      expect(eventSearchFilter('metadata', value)).toEqual({ query: value, queryMode: 'text' });
    }
    expect(eventSearchFilter('payload', 'data: {status: "paid", retry: false}, count: 0')).toEqual({ body: '{"data":{"status":"paid","retry":false},"count":0}' });
  });

  it('preserves punctuation and escaped quotes inside string values', () => {
    const text = 'literal {status: "paid"}, other: value, backslash: \\';
    const filter = eventSearchFilter('payload', 'note: ' + JSON.stringify(text) + ', items: [{item_id: 1}, {item_id: 2}]');
    expect(JSON.parse(filter.body!)).toEqual({ note: text, items: [{ item_id: 1 }, { item_id: 2 }] });
  });

  it('preserves quoted field names that cannot use shorthand and JSON value types', () => {
    const payload = { 'space key': { 'quoted"key': null, 'slash\\key': '\\"' }, amount: -1.5e3, enabled: true };
    expect(JSON.parse(eventSearchFilter('payload', JSON.stringify(payload)).body!)).toEqual(payload);
    expect(JSON.parse(eventSearchFilter('payload', 'items: [{"space key": null}, {retry-count: 0}], enabled: true').body!))
      .toEqual({ items: [{ 'space key': null }, { 'retry-count': 0 }], enabled: true });
  });

  it('keeps JSON-looking idempotency identifiers literal instead of interpreting them as payload', () => {
    const identifier = '{"delivery":"abc:123"}';
    expect(eventSearchFilter('key', identifier)).toEqual({ idempotencyKey: identifier });
    expect(eventSearchFilter('value', identifier)).toEqual({ idempotencyValue: identifier });
  });

  it('rejects mixed text, unquoted string values, scalars and empty payload filters', () => {
    for (const value of ['invoice.paid {"status":"paid"}', 'status:paid', 'null', '[]', '{}', '"paid"', 'status: "paid" trailing', 'data: {status: "paid"']) {
      expect(() => eventSearchFilter('payload', value)).toThrowError();
    }
  });

  it('looks up raw idempotency values literally', () => {
    expect(eventSearchFilter('value', ' qa-delivery-123 ')).toEqual({ idempotencyValue: 'qa-delivery-123' });
  });

  it('looks up stored idempotency keys separately from raw values', () => {
    expect(eventSearchFilter('key', ' stored-key-123 ')).toEqual({ idempotencyKey: 'stored-key-123' });
  });

  it('clears each search type for empty input', () => {
    for (const type of ['metadata', 'payload', 'value', 'key'] as const) {
      expect(eventSearchFilter(type, ' ')).toEqual({});
    }
  });
});
