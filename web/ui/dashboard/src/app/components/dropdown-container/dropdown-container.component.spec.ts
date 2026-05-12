import { ComponentFixture, TestBed } from '@angular/core/testing';

import { DropdownContainerComponent } from './dropdown-container.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('DropdownContainerComponent', () => {
  let component: DropdownContainerComponent;
  let fixture: ComponentFixture<DropdownContainerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, DropdownContainerComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(DropdownContainerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
