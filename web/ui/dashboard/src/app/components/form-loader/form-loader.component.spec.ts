import { ComponentFixture, TestBed } from '@angular/core/testing';

import { FormLoaderComponent } from './form-loader.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('FormLoaderComponent', () => {
  let component: FormLoaderComponent;
  let fixture: ComponentFixture<FormLoaderComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, FormLoaderComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(FormLoaderComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
