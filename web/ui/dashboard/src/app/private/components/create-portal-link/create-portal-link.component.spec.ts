import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreatePortalLinkComponent } from './create-portal-link.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('CreatePortalLinkComponent', () => {
  let component: CreatePortalLinkComponent;
  let fixture: ComponentFixture<CreatePortalLinkComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, CreatePortalLinkComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(CreatePortalLinkComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
