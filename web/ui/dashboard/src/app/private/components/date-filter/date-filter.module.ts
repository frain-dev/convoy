import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DateFilterComponent } from './date-filter.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';

@NgModule({
	declarations: [DateFilterComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, MatDatepickerModule, MatNativeDateModule],
	exports: [DateFilterComponent]
})
export class DateFilterModule {}
