import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EventCatalogComponent } from './event-catalog.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('EventCatalogComponent', () => {
  let component: EventCatalogComponent;
  let fixture: ComponentFixture<EventCatalogComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, EventCatalogComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(EventCatalogComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
