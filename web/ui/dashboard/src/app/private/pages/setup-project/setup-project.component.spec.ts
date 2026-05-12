import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SetupProjectComponent } from './setup-project.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('SetupProjectComponent', () => {
  let component: SetupProjectComponent;
  let fixture: ComponentFixture<SetupProjectComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, SetupProjectComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(SetupProjectComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
