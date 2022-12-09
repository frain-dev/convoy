import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreatePortalLinkComponent } from './create-portal-link.component';

describe('CreatePortalLinkComponent', () => {
  let component: CreatePortalLinkComponent;
  let fixture: ComponentFixture<CreatePortalLinkComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ CreatePortalLinkComponent ]
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
