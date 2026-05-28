import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateSubscriptionComponent } from './create-subscription.component';
import { RouterTestingModule } from '@angular/router/testing';
import { ActivatedRoute } from '@angular/router';
import { FormBuilder } from '@angular/forms';
import { PrivateService } from '../../private.service';
import { CreateSubscriptionService } from './create-subscription.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { FilterService } from './filter.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

describe('CreateSubscriptionComponent', () => {
  let component: CreateSubscriptionComponent;
  let fixture: ComponentFixture<CreateSubscriptionComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ CreateSubscriptionComponent ],
      imports: [ RouterTestingModule ],
      providers: [
        FormBuilder,
        { provide: ActivatedRoute, useValue: { snapshot: { params: {}, queryParams: {} } } },
        { provide: PrivateService, useValue: { getProjectDetails: { uid: 'project-id', type: 'incoming' }, getSubscriptions: jasmine.createSpy('getSubscriptions') } },
        { provide: CreateSubscriptionService, useValue: {} },
        { provide: LicensesService, useValue: {} },
        { provide: FilterService, useValue: {} },
        { provide: RbacService, useValue: { userPermission: async () => [] } }
      ]
    })
    .overrideComponent(CreateSubscriptionComponent, { set: { template: '' } })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(CreateSubscriptionComponent);
    component = fixture.componentInstance;
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('keeps the filter draft and disables it when an event type is unchecked', () => {
    component.subscriptionId = 'sub-id';
    component.selectedEventTypes = ['invoice.created'];
    component.filtersMap.set('invoice.created', {
      uid: 'filter-id',
      subscription_id: 'sub-id',
      event_type: 'invoice.created',
      enabled_at: '2026-05-28T00:00:00.000Z',
      headers: {},
      body: { kind: 'invoice' },
      query: {},
      path: {}
    });

    component.removeEventType(0, true);

    const filter = component.filtersMap.get('invoice.created');
    expect(filter).toBeTruthy();
    expect(filter?.enabled_at).toBeNull();
    expect(filter?.body).toEqual({ kind: 'invoice' });
  });

  it('disables specific filters instead of deleting them when All events is selected', () => {
    component.subscriptionId = 'sub-id';
    component.selectedEventTypes = ['invoice.created'];
    component.filtersMap.set('invoice.created', {
      uid: 'filter-id',
      subscription_id: 'sub-id',
      event_type: 'invoice.created',
      enabled_at: '2026-05-28T00:00:00.000Z',
      headers: {},
      body: { kind: 'invoice' },
      query: {},
      path: {}
    });

    component.toggleEventType('*');

    expect(component.selectedEventTypes).toEqual(['*']);
    expect(component.filtersMap.get('invoice.created')?.enabled_at).toBeNull();
    expect(component.filtersMap.get('invoice.created')?.body).toEqual({ kind: 'invoice' });
    expect(component.filtersMap.get('*')?.enabled_at).toEqual(jasmine.any(String));
  });

  it('updates the selected filter index after creating a new filter entry', () => {
    component.subscriptionId = 'sub-id';
    component.selectedEventTypes = ['invoice.created'];

    component.openFilterDialog('invoice.created');

    expect(component.selectedIndex).toBe(0);
    expect(component.filters[component.selectedIndex]?.event_type).toBe('invoice.created');
  });
});
