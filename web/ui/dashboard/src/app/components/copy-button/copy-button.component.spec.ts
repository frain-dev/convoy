import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CopyButtonComponent } from './copy-button.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('CopyButtonComponent', () => {
  let component: CopyButtonComponent;
  let fixture: ComponentFixture<CopyButtonComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, CopyButtonComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(CopyButtonComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
