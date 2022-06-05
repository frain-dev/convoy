import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';

import { PrivateRoutingModule } from './private-routing.module';
import { PrivateComponent } from './private.component';
import { CreateProjectComponent } from './components/create-project/create-project.component';

@NgModule({
	declarations: [PrivateComponent, CreateProjectComponent],
	imports: [CommonModule, PrivateRoutingModule]
})
export class PrivateModule {}
