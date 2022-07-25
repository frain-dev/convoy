import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TableHeadCellComponent } from './table-head-cell.component';

describe('TableHeadCellComponent', () => {
  let component: TableHeadCellComponent;
  let fixture: ComponentFixture<TableHeadCellComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ TableHeadCellComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(TableHeadCellComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
