import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateEndpointComponent } from './create-endpoint.component';
import { RouterTestingModule } from '@angular/router/testing';
import { EndpointsService } from '../../pages/project/endpoints/endpoints.service';

// A Power Automate Workflows webhook: explicit port, long path, and a sig query
// parameter. The form must accept this shape unchanged.
const TEAMS_WEBHOOK_URL = 'https://example.logic.azure.com:443/workflows/abc/triggers/manual/paths/invoke?sig=redacted';

describe('CreateEndpointComponent', () => {
  let component: CreateEndpointComponent;
  let fixture: ComponentFixture<CreateEndpointComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, CreateEndpointComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(CreateEndpointComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('should expose an empty teams_webhook_url control', () => {
    const control = component.addNewEndpointForm.get('teams_webhook_url');

    expect(control).toBeTruthy();
    expect(control?.value).toBe('');
    expect(control?.valid).toBeTrue();
  });

  it('should accept a workflows webhook url and reject a non-url', () => {
    const control = component.addNewEndpointForm.get('teams_webhook_url');

    control?.setValue(TEAMS_WEBHOOK_URL);
    expect(control?.valid).toBeTrue();

    control?.setValue('not-a-url');
    expect(control?.valid).toBeFalse();

    // Empty must stay valid: the field is optional and clearing it is how a user
    // turns the channel off.
    control?.setValue('');
    expect(control?.valid).toBeTrue();
  });

  // stubEndpoint makes getEndpointDetails resolve with the given endpoint body.
  const stubEndpoint = (data: Record<string, unknown>) => {
    spyOn(TestBed.inject(EndpointsService), 'getEndpoint').and.resolveTo({
      status: true,
      message: 'endpoint fetched successfully',
      data: { uid: 'endpoint-1', name: 'E1', url: 'https://e1.example.com', ...data }
    });
    component.endpointUid = 'endpoint-1';
  };

  it('should patch teams_webhook_url when loading an endpoint', async () => {
    stubEndpoint({ teams_webhook_url: TEAMS_WEBHOOK_URL });

    await component.getEndpointDetails();

    expect(component.addNewEndpointForm.get('teams_webhook_url')?.value).toBe(TEAMS_WEBHOOK_URL);
  });

  it('should open the Notifications section for a Teams-only endpoint', async () => {
    // Before this change the section was gated on support_email alone, so an
    // endpoint configured with only a webhook opened the edit form with its
    // notification channel hidden.
    const toggle = spyOn(component, 'toggleConfigForm').and.callThrough();

    stubEndpoint({ teams_webhook_url: TEAMS_WEBHOOK_URL });

    await component.getEndpointDetails();

    expect(toggle).toHaveBeenCalledWith('alert_config');
  });

  it('should leave the Notifications section closed when no channel is set', async () => {
    const toggle = spyOn(component, 'toggleConfigForm').and.callThrough();

    stubEndpoint({});

    await component.getEndpointDetails();

    expect(toggle).not.toHaveBeenCalledWith('alert_config');
  });
});
