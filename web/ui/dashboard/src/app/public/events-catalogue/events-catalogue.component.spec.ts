import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventsCatalogueComponent } from './events-catalogue.component';

describe('EventsCatalogueComponent', () => {
  let component: EventsCatalogueComponent;
  let fixture: ComponentFixture<EventsCatalogueComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ EventsCatalogueComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventsCatalogueComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
