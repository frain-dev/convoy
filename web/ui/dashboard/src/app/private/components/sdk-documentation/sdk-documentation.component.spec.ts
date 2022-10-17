import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SdkDocumentationComponent } from './sdk-documentation.component';

describe('SdkDocumentationComponent', () => {
  let component: SdkDocumentationComponent;
  let fixture: ComponentFixture<SdkDocumentationComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ SdkDocumentationComponent ]
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
