import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ConvoyDashboardComponent } from './convoy-dashboard.component';

describe('ConvoyDashboardComponent', () => {
  let component: ConvoyDashboardComponent;
  let fixture: ComponentFixture<ConvoyDashboardComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ ConvoyDashboardComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(ConvoyDashboardComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
