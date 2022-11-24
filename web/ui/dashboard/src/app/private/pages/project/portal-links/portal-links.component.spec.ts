import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PortalLinksComponent } from './portal-links.component';

describe('PortalLinksComponent', () => {
  let component: PortalLinksComponent;
  let fixture: ComponentFixture<PortalLinksComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ PortalLinksComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(PortalLinksComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
