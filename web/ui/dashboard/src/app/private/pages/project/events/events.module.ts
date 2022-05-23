import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { EventsComponent } from './events.component';
import { Routes, RouterModule } from '@angular/router';

const routes: Routes = [{ path: '', component: EventsComponent }];

@NgModule({
	declarations: [EventsComponent],
	imports: [CommonModule, RouterModule.forChild(routes)]
})
export class EventsModule {}
