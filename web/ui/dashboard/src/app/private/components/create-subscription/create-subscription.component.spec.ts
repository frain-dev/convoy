import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule, FormBuilder } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { NO_ERRORS_SCHEMA } from '@angular/core';

import { CreateSubscriptionComponent } from './create-subscription.component';
import { PrivateService } from '../../private.service';
import { CreateSubscriptionService } from './create-subscription.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { FilterService } from './filter.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

describe('CreateSubscriptionComponent', () => {
  let component: CreateSubscriptionComponent;
  let fixture: ComponentFixture<CreateSubscriptionComponent>;

  const mockPrivateService = {
    getProjectDetails: { type: 'outgoing', uid: 'test-project' },
    getEndpoints: () => Promise.resolve({ data: { content: [] } }),
    getSources: () => Promise.resolve({ data: { content: [] } }),
    getEventTypes: () => Promise.resolve({ data: [] }),
    getProjectStat: () => Promise.resolve({}),
    getSubscriptions: () => {}
  };

  const mockCreateSubscriptionService = {
    createSubscription: () => Promise.resolve({ data: { uid: 'sub-1' } }),
    updateSubscription: () => Promise.resolve({ data: {} }),
    getSubscriptionDetail: () => Promise.resolve({ data: {} })
  };

  const mockLicensesService = { hasLicense: () => true };

  const mockFilterService = {
    getFilters: () => Promise.resolve({ data: [] }),
    createFilters: () => Promise.resolve({}),
    bulkUpdateFilters: () => Promise.resolve({})
  };

  const mockRbacService = { userCanAccess: () => Promise.resolve(true) };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [CreateSubscriptionComponent],
      imports: [ReactiveFormsModule, RouterTestingModule],
      providers: [
        FormBuilder,
        { provide: PrivateService, useValue: mockPrivateService },
        { provide: CreateSubscriptionService, useValue: mockCreateSubscriptionService },
        { provide: LicensesService, useValue: mockLicensesService },
        { provide: FilterService, useValue: mockFilterService },
        { provide: RbacService, useValue: mockRbacService }
      ],
      schemas: [NO_ERRORS_SCHEMA]
    }).compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(CreateSubscriptionComponent);
    component = fixture.componentInstance;
    // Avoid calling fixture.detectChanges() so ngOnInit's complex async
    // flows do not run; tests set up component state directly.
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  describe('saveSubscription - incoming project filter injection fix', () => {
    beforeEach(() => {
      component.subscriptionForm.patchValue({ name: 'Test Subscription', endpoint_id: 'ep-1' });
    });

    it('should not inject wildcard event type for incoming projects when selectedEventTypes is empty', async () => {
      component.projectType = 'incoming';
      component.selectedEventTypes = [];

      await component.saveSubscription();

      expect(component.selectedEventTypes).toEqual([]);
      expect(component.selectedEventTypes).not.toContain('*');
    });

    it('should default selectedEventTypes to ["*"] for outgoing projects when empty', async () => {
      component.projectType = 'outgoing';
      component.selectedEventTypes = [];

      await component.saveSubscription();

      expect(component.selectedEventTypes).toContain('*');
    });
  });
});
