import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppDetailsComponent } from './app-details.component';
import { RouterModule, Routes } from '@angular/router';
import { ReactiveFormsModule } from '@angular/forms';
import { TooltipModule } from 'src/app/private/components/tooltip/tooltip.module';

const routes: Routes = [
	{
		path: '',
		component: AppDetailsComponent
	}
];
@NgModule({
	declarations: [AppDetailsComponent],
	imports: [CommonModule, ReactiveFormsModule, TooltipModule, RouterModule.forChild(routes)]
})
export class AppDetailsModule {}
