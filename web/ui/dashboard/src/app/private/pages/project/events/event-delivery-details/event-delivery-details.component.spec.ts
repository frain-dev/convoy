import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventDeliveryDetailsComponent } from './event-delivery-details.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventDeliveryDetailsComponent', () => {
  let component: EventDeliveryDetailsComponent;
  let fixture: ComponentFixture<EventDeliveryDetailsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventDeliveryDetailsComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(EventDeliveryDetailsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
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
