import { EventDeliveryDetailsComponent } from './event-delivery-details.component';

describe('EventDeliveryDetailsComponent', () => {
  let component: EventDeliveryDetailsComponent;
  let service: jasmine.SpyObj<any>;

  beforeEach(() => {
    service = jasmine.createSpyObj('EventDeliveryDetailsService', ['getEventDeliveryAttempts']);
    component = new EventDeliveryDetailsComponent({ snapshot: { params: { id: 'delivery' }, queryParams: {} } } as any, service, {} as any, {} as any);
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('appends older history without replacing the selected or latest attempt', async () => {
    const newest = { uid: 'newest' } as any;
    const older = { uid: 'older' } as any;
    service.getEventDeliveryAttempts.and.resolveTo({ data: { content: [older, newest], next_cursor: 'older' } });
    await component.getEventDeliveryAttempts('delivery');
    expect(component.eventDeliveryAtempts).toEqual([newest, older]);
    component.selectedDeliveryAttempt = older;
    const oldest = { uid: 'oldest' } as any;
    service.getEventDeliveryAttempts.and.resolveTo({ data: { content: [oldest], next_cursor: '' } });
    await component.getEventDeliveryAttempts('delivery', true);
    expect(service.getEventDeliveryAttempts).toHaveBeenCalledWith({ eventId: 'delivery', before: 'older' });
    expect(component.eventDeliveryAtempts).toEqual([newest, older, oldest]);
    expect(component.selectedDeliveryAttempt).toBe(older);
    expect(component.eventDeliveryAtempt).toBe(newest);
    expect(component.nextAttemptsCursor).toBe('');
  });

  it('keeps loaded attempts and cursor on failure so loading can be retried', async () => {
    const attempt = { uid: 'loaded' } as any;
    component.eventDeliveryAtempts = [attempt];
    component.nextAttemptsCursor = 'loaded';
    service.getEventDeliveryAttempts.and.rejectWith(new Error('offline'));
    await component.getEventDeliveryAttempts('delivery', true);
    expect(component.eventDeliveryAtempts).toEqual([attempt]);
    expect(component.nextAttemptsCursor).toBe('loaded');
    expect(component.attemptsError).toContain('Could not load');
    expect(component.isLoadingOlderAttempts).toBeFalse();
  });

  it('does not fetch older history while a request is running or after the final page', async () => {
    await component.getEventDeliveryAttempts('delivery', true);
    component.nextAttemptsCursor = 'cursor';
    component.isLoadingOlderAttempts = true;
    await component.getEventDeliveryAttempts('delivery', true);
    expect(service.getEventDeliveryAttempts).not.toHaveBeenCalled();
  });

  it('loads automatically when older history enters the scroll area and disconnects on destroy', () => {
    let callback!: IntersectionObserverCallback;
    const observer = jasmine.createSpyObj('IntersectionObserver', ['observe', 'disconnect']);
    spyOn(window, 'IntersectionObserver').and.callFake(function (cb: IntersectionObserverCallback, options?: IntersectionObserverInit) {
      callback = cb;
      expect(options?.root).toBe(root);
      return observer;
    });
    const root = document.createElement('div');
    const sentinel = document.createElement('button');
    root.appendChild(sentinel);
    const load = spyOn(component, 'getEventDeliveryAttempts').and.resolveTo();
    component.olderAttempts = { nativeElement: sentinel };
    expect(observer.observe).toHaveBeenCalledWith(sentinel);
    callback([{ isIntersecting: false } as IntersectionObserverEntry], observer);
    expect(load).not.toHaveBeenCalled();
    callback([{ isIntersecting: true } as IntersectionObserverEntry], observer);
    expect(load).toHaveBeenCalledOnceWith('delivery', true);
    component.attemptsError = 'offline';
    callback([{ isIntersecting: true } as IntersectionObserverEntry], observer);
    expect(load).toHaveBeenCalledTimes(1);
    component.ngOnDestroy();
    expect(observer.disconnect).toHaveBeenCalled();
    component.attemptsError = '';
    callback([{ isIntersecting: true } as IntersectionObserverEntry], observer);
    expect(load).toHaveBeenCalledTimes(1);
  });

  describe('deliveredToUrl', () => {
    it('prefers delivery target_url over attempt url', () => {
      component.eventDelsDetails = { target_url: 'https://old.example/hook' } as any;
      component.eventDeliveryAtempt = { url: 'https://attempt.example/hook' } as any;

      expect(component.deliveredToUrl()).toBe('https://old.example/hook');
    });

    it('uses latest attempt url when target_url is empty', () => {
      component.eventDelsDetails = {
        target_url: '',
        endpoint_metadata: { url: 'https://live.example/hook' }
      } as any;
      component.eventDeliveryAtempt = { url: 'https://attempt.example/hook' } as any;

      expect(component.deliveredToUrl()).toBe('https://attempt.example/hook');
    });

    it('does not fall back to endpoint_metadata.url', () => {
      component.eventDelsDetails = {
        endpoint_metadata: { url: 'https://live.example/hook' }
      } as any;

      expect(component.deliveredToUrl()).toBe('');
    });

    it('uses selected attempt when user picks an older attempt', () => {
      component.eventDelsDetails = {} as any;
      component.eventDeliveryAtempt = { url: 'https://newest.example/hook' } as any;
      component.selectedDeliveryAttempt = { url: 'https://older.example/hook' } as any;

      expect(component.deliveredToUrl()).toBe('https://older.example/hook');
    });
  });
});
