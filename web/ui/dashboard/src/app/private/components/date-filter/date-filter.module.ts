import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DateFilterComponent } from './date-filter.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DropdownComponent } from 'src/stories/dropdown/dropdown.component';

@NgModule({
	declarations: [DateFilterComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, RouterModule, ButtonComponent, DropdownComponent],
	exports: [DateFilterComponent]
})
export class DateFilterModule {}
