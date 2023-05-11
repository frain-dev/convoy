import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MetaEventsComponent } from './meta-events.component';

describe('MetaEventsComponent', () => {
  let component: MetaEventsComponent;
  let fixture: ComponentFixture<MetaEventsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ MetaEventsComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(MetaEventsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
