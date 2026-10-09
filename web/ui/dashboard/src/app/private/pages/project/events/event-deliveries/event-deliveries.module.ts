import { NgModule } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterModule } from '@angular/router';
import { EventDeliveriesComponent } from './event-deliveries.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';

@NgModule({
	declarations: [EventDeliveriesComponent],
	imports: [CommonModule, FormsModule, RouterModule, DropdownComponent, DropdownOptionDirective, DialogDirective],
	exports: [EventDeliveriesComponent],
	providers: [DatePipe]
})
export class EventDeliveriesModule {}
