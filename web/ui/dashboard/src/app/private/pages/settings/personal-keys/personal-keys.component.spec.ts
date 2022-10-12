import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PersonalKeysComponent } from './personal-keys.component';

describe('PersonalKeysComponent', () => {
  let component: PersonalKeysComponent;
  let fixture: ComponentFixture<PersonalKeysComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ PersonalKeysComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(PersonalKeysComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
