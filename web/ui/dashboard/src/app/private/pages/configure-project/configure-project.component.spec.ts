import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ConfigureProjectComponent } from './configure-project.component';

describe('ConfigureProjectComponent', () => {
  let component: ConfigureProjectComponent;
  let fixture: ComponentFixture<ConfigureProjectComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ ConfigureProjectComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ConfigureProjectComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
