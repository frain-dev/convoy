import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TableHeadComponent } from './table-head.component';

describe('TableHeadComponent', () => {
  let component: TableHeadComponent;
  let fixture: ComponentFixture<TableHeadComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ TableHeadComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(TableHeadComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
