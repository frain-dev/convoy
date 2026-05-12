import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PrismComponent } from './prism.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('PrismComponent', () => {
  let component: PrismComponent;
  let fixture: ComponentFixture<PrismComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, PrismComponent ]
    })
    .compileComponents();
  });

  beforeEach(() => {
    fixture = TestBed.createComponent(PrismComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
