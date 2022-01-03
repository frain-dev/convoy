import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ConvoyAppComponent } from './convoy-app.component';

describe('ConvoyAppComponent', () => {
  let component: ConvoyAppComponent;
  let fixture: ComponentFixture<ConvoyAppComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      declarations: [ ConvoyAppComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(ConvoyAppComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
