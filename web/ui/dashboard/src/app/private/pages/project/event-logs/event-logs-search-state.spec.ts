import { fakeAsync, tick } from '@angular/core/testing';
import { EventLogsComponent } from './event-logs.component';

describe('Event log search mode changes', () => {
  let logs: EventLogsComponent;

  beforeEach(() => {
    const general = {
      addFilterToURL: jasmine.createSpy('addFilterToURL').and.callFake((params: Record<string, unknown>) =>
        Object.fromEntries(Object.entries(params).filter(([, value]) => value !== '' && value !== undefined)))
    };
    logs = new EventLogsComponent({} as any, general as any, { snapshot: { params: {} } } as any,
      {} as any, {} as any, {} as any, { hasLicense: () => true } as any, {} as any, {} as any);
    spyOn(logs, 'refreshEvents');
    logs.queryParams = { sourceId: 'source-1', startDate: '2026-10-01', endDate: '2026-10-20' };
  });

  it('restores legacy JSON query links as payload filters', fakeAsync(() => {
    (logs as any).syncFiltersFromRoute({ query: '{"status":"paid"}' });
    expect(logs.searchType).toBe('payload');
    expect(logs.searchInput).toBe('{"status":"paid"}');
    expect(logs.queryParams.query).toBeUndefined();
    logs.searchInput = 'status: "pending"';
    logs.onSearch();
    tick(400);
    expect(logs.queryParams.body).toBe('{"status":"pending"}');
    expect(logs.queryParams.queryMode).toBeUndefined();
  }));

  it('keeps both parts of a legacy mixed search visible', () => {
    (logs as any).syncFiltersFromRoute({ query: 'invoice.paid {"status":"paid"}' });
    expect(logs.searchType).toBe('payload');
    expect(logs.queryParams.body).toBe('{"status":"paid"}');
    expect(logs.queryParams.query).toBe('invoice.paid');
    expect(logs.additionalSearchFilters).toEqual([{ type: 'metadata', label: 'Event metadata', value: 'invoice.paid' }] as any);
  });

  it('does not reinterpret explicit text links containing JSON', () => {
    (logs as any).syncFiltersFromRoute({ query: '{"status":"paid"}', queryMode: 'text' });
    expect(logs.searchType).toBe('metadata');
    expect(logs.queryParams.body).toBeUndefined();
  });

  it('cancels a pending search and clears old filters when switching type', fakeAsync(() => {
    logs.searchInput = 'invoice.paid';
    logs.onSearch();
    logs.selectSearchType('value');
    tick(500);

    expect(logs.queryParams).toEqual({ sourceId: 'source-1', startDate: '2026-10-01', endDate: '2026-10-20' });
    expect(logs.searchInput).toBe('');
    expect(logs.refreshEvents).toHaveBeenCalledTimes(1);
  }));

  it('submits JSON through the payload parameter and restores metadata mode explicitly', fakeAsync(() => {
    logs.selectSearchType('payload');
    logs.searchInput = '{"status":"paid"}';
    logs.onSearch();
    tick(400);
    expect(logs.queryParams.body).toBe('{"status":"paid"}');
    expect(logs.queryParams.query).toBeUndefined();

    logs.selectSearchType('metadata');
    logs.searchInput = '{"status":"paid"}';
    logs.onSearch();
    tick(400);
    expect(logs.queryParams.queryMode).toBe('text');
    expect(logs.queryParams.body).toBeUndefined();
  }));

  it('does not send malformed JSON or silently fall back to metadata search', fakeAsync(() => {
    logs.searchType = 'payload';
    logs.searchInput = '{status:paid}';
    logs.onSearch();
    tick(500);
    expect(logs.searchValidationError).toContain('double quotes around string values');
    expect(logs.refreshEvents).not.toHaveBeenCalled();
  }));

  it('clears the stored key before looking up a raw value', fakeAsync(() => {
    logs.selectSearchType('key');
    logs.searchInput = 'stored-key';
    logs.onSearch();
    tick(400);
    expect(logs.queryParams.idempotencyKey).toBe('stored-key');

    logs.selectSearchType('value');
    logs.searchInput = 'raw-value';
    logs.onSearch();
    tick(400);
    expect(logs.queryParams.idempotencyValue).toBe('raw-value');
    expect(logs.queryParams.idempotencyKey).toBeUndefined();
    expect(logs.queryParams.sourceId).toBe('source-1');
  }));

  it('restores the key search type and value from a shared URL', () => {
    (logs as any).syncFiltersFromRoute({ idempotencyKey: 'stored-key' });
    expect(logs.searchType).toBe('key');
    expect(logs.searchInput).toBe('stored-key');
  });

  it('restores each explicit search mode from a shared URL', () => {
    const cases = [
      { params: { query: '{"status":"paid"}', queryMode: 'text' }, type: 'metadata', input: '{"status":"paid"}' },
      { params: { body: '{"status":"paid"}' }, type: 'payload', input: '{"status":"paid"}' },
      { params: { idempotencyValue: 'raw-delivery' }, type: 'value', input: 'raw-delivery' },
      { params: {}, type: 'metadata', input: '' }
    ];
    for (const testCase of cases) {
      (logs as any).syncFiltersFromRoute(testCase.params);
      expect(logs.searchType).toBe(testCase.type);
      expect(logs.searchInput).toBe(testCase.input);
    }
  });

  it('only submits the latest keystrokes after the debounce', fakeAsync(() => {
    logs.searchInput = 'invoice';
    logs.onSearch();
    tick(200);
    logs.searchInput = 'invoice.paid';
    logs.onSearch();
    tick(399);
    expect(logs.refreshEvents).not.toHaveBeenCalled();
    tick(1);
    expect(logs.queryParams.query).toBe('invoice.paid');
    expect(logs.refreshEvents).toHaveBeenCalledTimes(1);
  }));

  it('cancels a previously valid pending payload when the user makes it invalid', fakeAsync(() => {
    logs.searchType = 'payload';
    logs.searchInput = 'status: "paid"';
    logs.onSearch();
    tick(200);
    logs.searchInput = 'status: "';
    logs.onSearch();
    tick(500);
    expect(logs.searchValidationError).toBeTruthy();
    expect(logs.refreshEvents).not.toHaveBeenCalled();
    expect(logs.queryParams.body).toBeUndefined();
  }));

  it('does not submit a pending search after leaving the event log page', fakeAsync(() => {
    logs.searchInput = 'invoice.paid';
    logs.onSearch();
    logs.ngOnDestroy();
    tick(500);
    expect(logs.refreshEvents).not.toHaveBeenCalled();
  }));

  it('does not restore a pending query after clearing filters', fakeAsync(() => {
    logs.searchInput = 'invoice.paid';
    logs.onSearch();
    logs.clearAllFilters();
    tick(500);
    expect(logs.queryParams.query).toBeUndefined();
    expect(logs.searchInput).toBe('');
  }));
});
