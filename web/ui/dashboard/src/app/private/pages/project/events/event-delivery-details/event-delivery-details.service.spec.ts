import { EventDeliveryDetailsService } from './event-delivery-details.service';

describe('EventDeliveryDetailsService', () => {
  let service: EventDeliveryDetailsService;
  let http: jasmine.SpyObj<any>;

  beforeEach(() => {
    http = jasmine.createSpyObj('HttpService', ['request']);
    http.request.and.resolveTo({ data: { content: [], next_cursor: '' } });
    service = new EventDeliveryDetailsService(http);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  it('opts into pagination for the initial page', async () => {
    await service.getEventDeliveryAttempts({ eventId: 'delivery' });
    expect(http.request).toHaveBeenCalledWith({ url: '/eventdeliveries/delivery/deliveryattempts?paginated=true', method: 'get', level: 'org_project' });
  });

  it('encodes the exclusive cursor for subsequent pages', async () => {
    await service.getEventDeliveryAttempts({ eventId: 'delivery', before: 'cursor&other=true' });
    expect(http.request).toHaveBeenCalledWith({ url: '/eventdeliveries/delivery/deliveryattempts?paginated=true&before=cursor%26other%3Dtrue', method: 'get', level: 'org_project' });
  });
});
