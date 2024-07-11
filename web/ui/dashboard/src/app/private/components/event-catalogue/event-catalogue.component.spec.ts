import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventCatalogueComponent } from './event-catalogue.component';

describe('EventCatalogueComponent', () => {
  let component: EventCatalogueComponent;
  let fixture: ComponentFixture<EventCatalogueComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ EventCatalogueComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventCatalogueComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
