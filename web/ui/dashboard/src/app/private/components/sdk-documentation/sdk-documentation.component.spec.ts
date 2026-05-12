import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SdkDocumentationComponent } from './sdk-documentation.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('SdkDocumentationComponent', () => {
  let component: SdkDocumentationComponent;
  let fixture: ComponentFixture<SdkDocumentationComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, SdkDocumentationComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(SdkDocumentationComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
